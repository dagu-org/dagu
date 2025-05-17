package localproc

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Error messages
var (
	ErrHeartbeatAlreadyStarted = fmt.Errorf("heartbeat already started")
)

var _ models.Proc = (*Proc)(nil)

// Proc is a struct that implements the Proc interface.
type Proc struct {
	fileName string
	started  atomic.Bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	wg       sync.WaitGroup
}

// NewProc creates a new instance of Proc with the specified file name.
func NewProc(file string) *Proc {
	return &Proc{
		fileName: file,
	}
}

// Stop implements models.Proc.
func (p *Proc) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started.Load() {
		return fmt.Errorf("heartbeat not started")
	}
	if p.cancel != nil {
		p.cancel()
	}
	// Wait for the heartbeat goroutine to finish
	p.wg.Wait()
	return nil
}

// Start implements models.Proc.
func (p *Proc) Start(ctx context.Context) error {
	if !p.started.CompareAndSwap(false, true) {
		return ErrHeartbeatAlreadyStarted
	}

	// Ensure the directory exists
	dir := filepath.Dir(p.fileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fd, err := os.OpenFile(p.fileName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	if err != nil {
		p.started.Store(false)
		return err
	}

	hbCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	// Write the initial heartbeat timestamp
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().UnixNano()))
	if _, err := fd.WriteAt(buf, 0); err != nil {
		_ = fd.Close()
		if err := os.Remove(p.fileName); err != nil {
			logger.Error(ctx, "Failed to remove heartbeat file", "err", err)
		}
		p.started.Store(false)
		return err
	}
	_ = fd.Sync()

	p.wg.Add(1)

	// Start the heartbeat goroutine
	// It will write the current timestamp to the file every 15 seconds
	// and flush the file every 30 seconds.
	// The goroutine will stop when the context is canceled.
	// A proc file can be deemed stale if it has not been updated for 45 seconds
	// and also the content of the timestamp is older than 45 seconds.
	go func() {
		defer func() {
			_ = fd.Close()
			if err := os.Remove(p.fileName); err != nil {
				logger.Error(ctx, "Failed to remove heartbeat file", "err", err)
			}
			p.cancel = nil
			p.started.Store(false)
			p.wg.Done()
		}()

		ticker := time.NewTicker(15 * time.Second)
		flush := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer flush.Stop()

		buf := make([]byte, 8)

		for {
			select {
			case <-ticker.C:
				binary.BigEndian.PutUint64(buf, uint64(time.Now().UnixNano()))
				if _, err := fd.WriteAt(buf, 0); err != nil {
					logger.Error(ctx, "Failed to write heartbeat", "err", err)
				}

			case <-flush.C:
				_ = fd.Sync()

			case <-hbCtx.Done():
				_ = fd.Sync()
				return
			}
		}
	}()

	return nil
}
