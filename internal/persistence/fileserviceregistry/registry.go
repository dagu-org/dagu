package fileserviceregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// registry implements models.ServiceRegistry using file-based discovery
type registry struct {
	baseDir string
	finders map[models.ServiceName]*finder
	mu      sync.RWMutex

	// For this instance's registration
	instanceInfo      *instanceInfo
	fileName          string // File name for this instance
	serviceName       models.ServiceName
	instanceMu        sync.Mutex
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	heartbeatInterval time.Duration
}

// New creates a new file-based service registry
func New(discoveryDir string) *registry {
	return &registry{
		baseDir:           discoveryDir,
		finders:           make(map[models.ServiceName]*finder),
		heartbeatInterval: 10 * time.Second, // default
	}
}

// Register begins monitoring services and registers this instance
func (r *registry) Register(ctx context.Context, serviceName models.ServiceName, hostInfo models.HostInfo) error {
	r.instanceMu.Lock()
	defer r.instanceMu.Unlock()

	if r.cancel != nil {
		return fmt.Errorf("registry already started")
	}

	logger.Info(ctx, "Starting service registry",
		"service_name", serviceName,
		"instance_id", hostInfo.ID,
		"address", hostInfo.HostPort)

	// Ensure base directory exists
	if err := os.MkdirAll(r.baseDir, 0750); err != nil {
		return fmt.Errorf("failed to create discovery directory: %w", err)
	}

	// Set instance info
	r.serviceName = serviceName
	r.instanceInfo = &instanceInfo{
		ID:       hostInfo.ID,
		HostPort: hostInfo.HostPort,
		PID:      os.Getpid(),
	}

	// Write initial instance file
	r.fileName = r.instanceFilePath()
	if err := writeInstanceFile(r.fileName, r.instanceInfo); err != nil {
		return fmt.Errorf("failed to write instance file: %w", err)
	}

	// Start heartbeat for this instance
	if err := r.startHeartbeat(ctx, r.heartbeatInterval); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	return nil
}

// GetServiceMembers returns the list of active hosts for the given service.
// This method combines service resolution and member discovery.
func (r *registry) GetServiceMembers(ctx context.Context, serviceName models.ServiceName) ([]models.HostInfo, error) {
	finder := r.getFinder(serviceName)
	return finder.members(ctx)
}

// getFinder returns the service finder for a specific service (internal method)
func (r *registry) getFinder(serviceName models.ServiceName) *finder {
	r.mu.RLock()
	f, exists := r.finders[serviceName]
	r.mu.RUnlock()

	if !exists {
		r.mu.Lock()
		// Double-check after acquiring write lock
		if f, exists = r.finders[serviceName]; !exists {
			f = newFinder(r.baseDir, serviceName)
			r.finders[serviceName] = f
		}
		r.mu.Unlock()
	}

	return f
}

// Unregister stops the service registry
func (r *registry) Unregister(ctx context.Context) {
	r.instanceMu.Lock()

	if r.cancel == nil {
		// Already stopped
		r.instanceMu.Unlock()
		return
	}

	logger.Info(ctx, "Stopping service registry",
		"service_name", r.serviceName,
		"instance_id", r.instanceInfo.ID,
		"address", r.instanceInfo.HostPort)

	// Cancel the context to stop background goroutines
	cancel := r.cancel
	r.cancel = nil

	// Stop this instance's registration if active
	if r.instanceInfo != nil {
		// Remove instance file
		if err := removeInstanceFile(r.fileName); err != nil {
			logger.Error(ctx, "Failed to remove instance file", "err", err, "file", r.instanceInfo.ID)
		}
		r.instanceInfo = nil
	}
	r.instanceMu.Unlock()

	// Cancel context after releasing mutex to avoid deadlock
	cancel()

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-time.After(5 * time.Second):
		// Force shutdown after timeout
		logger.Warn(ctx, "Timeout waiting for registry shutdown")
	}
}

// instanceFilePath returns the full path for this instance's file
func (r *registry) instanceFilePath() string {
	return filepath.Join(r.baseDir, string(r.serviceName), fmt.Sprintf("%s.json", fileutil.SafeName(r.instanceInfo.ID)))
}

// startHeartbeat starts a background goroutine to update heartbeat for this instance
// Must be called with instanceMu held
func (r *registry) startHeartbeat(ctx context.Context, interval time.Duration) error {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.instanceMu.Lock()
				if r.instanceInfo == nil {
					r.instanceMu.Unlock()
					continue
				}

				// Update file modification time for heartbeat
				filename := r.fileName

				// Check if file exists, recreate if needed
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					// File doesn't exist, recreate it
					if err := writeInstanceFile(filename, r.instanceInfo); err != nil {
						logger.Error(ctx, "Failed to recreate instance file", "err", err, "file", filename)
						r.instanceMu.Unlock()
						continue
					}
				}

				// Update modification time
				now := time.Now()
				if err := os.Chtimes(filename, now, now); err != nil {
					logger.Error(ctx, "Failed to update heartbeat", "err", err, "file", filename)
				}
				r.instanceMu.Unlock()
			}
		}
	}()
	return nil
}
