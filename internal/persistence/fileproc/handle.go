package fileproc

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Error messages
var (
	ErrHeartbeatAlreadyStarted = fmt.Errorf("heartbeat already started")
)

var _ execution.ProcHandle = (*ProcHandle)(nil)

// ProcHandle is a struct that implements the ProcHandle interface.
type ProcHandle struct {
	fileName string
	started  atomic.Bool
	canceled atomic.Bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	wg       sync.WaitGroup
	meta     execution.ProcMeta
}

// GetMeta implements models.ProcHandle.
func (p *ProcHandle) GetMeta() execution.ProcMeta {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.meta
}

// NewProcHandler creates a new instance of Proc with the specified file name.
func NewProcHandler(file string, meta execution.ProcMeta) *ProcHandle {
	return &ProcHandle{
		fileName: file,
		meta:     meta,
	}
}

// Stop implements models.Proc.
func (p *ProcHandle) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started.Load() {
		return fmt.Errorf("heartbeat not started")
	}
	if p.canceled.CompareAndSwap(false, true) {
		if p.cancel != nil {
			p.cancel()
		}
		// Wait for the heartbeat goroutine to finish
		p.wg.Wait()
	}
	// Remove parent directory if it's empty
	_ = os.Remove(filepath.Dir(p.fileName))
	return nil
}

// startHeartbeat starts the heartbeat for the process.
func (p *ProcHandle) startHeartbeat(ctx context.Context) error {
	if !p.started.CompareAndSwap(false, true) {
		return ErrHeartbeatAlreadyStarted
	}

	// Ensure the directory exists
	dir := filepath.Dir(p.fileName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fd, err := os.OpenFile(p.fileName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		p.started.Store(false)
		return err
	}

	hbCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.canceled.Store(false)

	// Write the initial heartbeat timestamp in binary format
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix())) // nolint:gosec
	if _, err := fd.WriteAt(buf, 0); err != nil {
		_ = fd.Close()
		if err := os.Remove(p.fileName); err != nil {
			logger.Error(ctx, "Failed to remove heartbeat file", tag.Error, err)
		}

		// If the directory is empty after removing the file, remove the directory as well
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			if err := os.Remove(dir); err != nil {
				logger.Info(ctx, "Failed to remove empty heartbeat directory", tag.Error, err)
			}
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
		// recovery
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = fmt.Errorf("%s", v)
				default:
					err = fmt.Errorf("%v", v)
				}
				logger.Error(ctx, "Heartbeat goroutine panicked", tag.Error, err)
			}
		}()

		defer func() {
			_ = fd.Close()
			if err := os.Remove(p.fileName); err != nil {
				logger.Error(ctx, "Failed to remove heartbeat file", tag.Error, err)
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
			case <-hbCtx.Done():
				_ = fd.Sync()
				return
			case <-ticker.C:
				binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix())) // nolint:gosec
				if _, err := fd.WriteAt(buf, 0); err != nil {
					logger.Error(ctx, "Failed to write heartbeat", tag.Error, err)
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
