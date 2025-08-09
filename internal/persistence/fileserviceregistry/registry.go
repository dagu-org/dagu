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

// Monitor implements models.ServiceRegistry using file-based discovery
type Monitor struct {
	baseDir   string
	resolvers map[models.ServiceName]*resolver
	mu        sync.RWMutex

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
func New(discoveryDir string) *Monitor {
	return &Monitor{
		baseDir:           discoveryDir,
		resolvers:         make(map[models.ServiceName]*resolver),
		heartbeatInterval: 10 * time.Second, // default
	}
}

// Register begins monitoring services and registers this instance
func (m *Monitor) Register(ctx context.Context, serviceName models.ServiceName, hostInfo models.HostInfo) error {
	m.instanceMu.Lock()
	defer m.instanceMu.Unlock()

	if m.cancel != nil {
		return fmt.Errorf("registry already started")
	}

	logger.Info(ctx, "Starting service registry",
		"service_name", serviceName,
		"instance_id", hostInfo.ID,
		"address", hostInfo.HostPort)

	// Ensure base directory exists
	if err := os.MkdirAll(m.baseDir, 0750); err != nil {
		return fmt.Errorf("failed to create discovery directory: %w", err)
	}

	// Set instance info
	m.serviceName = serviceName
	m.instanceInfo = &instanceInfo{
		ID:       hostInfo.ID,
		HostPort: hostInfo.HostPort,
		PID:      os.Getpid(),
	}

	// Write initial instance file
	m.fileName = m.instanceFilePath()
	if err := writeInstanceFile(m.fileName, m.instanceInfo); err != nil {
		return fmt.Errorf("failed to write instance file: %w", err)
	}

	// Start heartbeat for this instance
	if err := m.startHeartbeat(ctx, m.heartbeatInterval); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	return nil
}

// Resolver returns the service resolver for a specific service
func (m *Monitor) Resolver(_ context.Context, serviceName models.ServiceName) models.ServiceResolver {
	m.mu.RLock()
	r, exists := m.resolvers[serviceName]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		// Double-check after acquiring write lock
		if r, exists = m.resolvers[serviceName]; !exists {
			r = newResolver(m.baseDir, serviceName)
			m.resolvers[serviceName] = r
		}
		m.mu.Unlock()
	}

	return r
}

// Unregister stops the service registry
func (m *Monitor) Unregister(ctx context.Context) {
	m.instanceMu.Lock()

	if m.cancel == nil {
		// Already stopped
		m.instanceMu.Unlock()
		return
	}

	logger.Info(ctx, "Stopping service registry",
		"service_name", m.serviceName,
		"instance_id", m.instanceInfo.ID,
		"address", m.instanceInfo.HostPort)

	// Cancel the context to stop background goroutines
	cancel := m.cancel
	m.cancel = nil

	// Stop this instance's registration if active
	if m.instanceInfo != nil {
		// Remove instance file
		if err := removeInstanceFile(m.fileName); err != nil {
			logger.Error(ctx, "Failed to remove instance file", "err", err, "file", m.instanceInfo.ID)
		}
		m.instanceInfo = nil
	}
	m.instanceMu.Unlock()

	// Cancel context after releasing mutex to avoid deadlock
	cancel()

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
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
func (m *Monitor) instanceFilePath() string {
	return filepath.Join(m.baseDir, string(m.serviceName), fmt.Sprintf("%s.json", fileutil.SafeName(m.instanceInfo.ID)))
}

// startHeartbeat starts a background goroutine to update heartbeat for this instance
// Must be called with instanceMu held
func (m *Monitor) startHeartbeat(ctx context.Context, interval time.Duration) error {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.instanceMu.Lock()
				if m.instanceInfo == nil {
					m.instanceMu.Unlock()
					continue
				}

				// Update file modification time for heartbeat
				filename := m.fileName

				// Check if file exists, recreate if needed
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					// File doesn't exist, recreate it
					if err := writeInstanceFile(filename, m.instanceInfo); err != nil {
						logger.Error(ctx, "Failed to recreate instance file", "err", err, "file", filename)
						m.instanceMu.Unlock()
						continue
					}
				}

				// Update modification time
				now := time.Now()
				if err := os.Chtimes(filename, now, now); err != nil {
					logger.Error(ctx, "Failed to update heartbeat", "err", err, "file", filename)
				}
				m.instanceMu.Unlock()
			}
		}
	}()
	return nil
}
