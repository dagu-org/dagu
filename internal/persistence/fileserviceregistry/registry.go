package fileserviceregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// registrationInfo holds information about a single service registration
type registrationInfo struct {
	instanceInfo *instanceInfo
	fileName     string
	serviceName  execution.ServiceName
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// registry implements models.ServiceRegistry using file-based service registry
type registry struct {
	baseDir string
	finders map[execution.ServiceName]*finder
	mu      sync.RWMutex

	// Map of service registrations (can have multiple)
	registrations     map[execution.ServiceName]*registrationInfo
	registrationsMu   sync.Mutex
	heartbeatInterval time.Duration
}

// New creates a new file-based service registry
func New(serviceRegistryDir string) *registry {
	return &registry{
		baseDir:           serviceRegistryDir,
		finders:           make(map[execution.ServiceName]*finder),
		registrations:     make(map[execution.ServiceName]*registrationInfo),
		heartbeatInterval: 10 * time.Second, // default
	}
}

// Register begins monitoring services and registers this instance
func (r *registry) Register(ctx context.Context, serviceName execution.ServiceName, hostInfo execution.HostInfo) error {
	r.registrationsMu.Lock()
	defer r.registrationsMu.Unlock()

	// Check if this service is already registered
	if _, exists := r.registrations[serviceName]; exists {
		return fmt.Errorf("service %s already registered", serviceName)
	}

	logger.Info(ctx, "Starting service registry",
		"service_name", serviceName,
		"instance_id", hostInfo.ID,
		"host", hostInfo.Host,
		"port", hostInfo.Port,
		"status", hostInfo.Status.String())

	// Ensure base directory exists
	if err := os.MkdirAll(r.baseDir, 0750); err != nil {
		return fmt.Errorf("failed to create service registry directory: %w", err)
	}

	// Create registration info
	reg := &registrationInfo{
		serviceName: serviceName,
		instanceInfo: &instanceInfo{
			ID:        hostInfo.ID,
			Host:      hostInfo.Host,
			Port:      hostInfo.Port,
			PID:       os.Getpid(),
			Status:    hostInfo.Status,
			StartedAt: time.Now(),
		},
	}

	// Generate file path
	reg.fileName = filepath.Join(r.baseDir, string(serviceName), fmt.Sprintf("%s.json", fileutil.SafeName(hostInfo.ID)))

	// Write initial instance file
	if err := writeInstanceFile(reg.fileName, reg.instanceInfo); err != nil {
		return fmt.Errorf("failed to write instance file: %w", err)
	}

	// Store registration
	r.registrations[serviceName] = reg

	// Start heartbeat for this instance
	if err := r.startHeartbeat(ctx, serviceName, r.heartbeatInterval); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	return nil
}

// GetServiceMembers returns the list of active hosts for the given service.
// This method combines service resolution and member lookup.
func (r *registry) GetServiceMembers(ctx context.Context, serviceName execution.ServiceName) ([]execution.HostInfo, error) {
	finder := r.getFinder(serviceName)
	return finder.members(ctx)
}

// getFinder returns the service finder for a specific service (internal method)
func (r *registry) getFinder(serviceName execution.ServiceName) *finder {
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

// Unregister stops all service registrations
func (r *registry) Unregister(ctx context.Context) {
	r.registrationsMu.Lock()
	registrations := r.registrations
	r.registrations = make(map[execution.ServiceName]*registrationInfo)
	r.registrationsMu.Unlock()

	// Stop all registrations
	for serviceName, reg := range registrations {
		logger.Info(ctx, "Stopping service registry",
			"service_name", serviceName,
			"instance_id", reg.instanceInfo.ID,
			"host", reg.instanceInfo.Host,
			"port", reg.instanceInfo.Port)

		// Cancel the context to stop background goroutines
		if reg.cancel != nil {
			reg.cancel()
		}

		// Remove instance file
		if err := removeInstanceFile(reg.fileName); err != nil {
			logger.Error(ctx, "Failed to remove instance file", "err", err, "file", reg.instanceInfo.ID)
		}

		// Wait for background goroutines with timeout
		done := make(chan struct{})
		go func() {
			reg.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Clean shutdown
		case <-time.After(5 * time.Second):
			// Force shutdown after timeout
			logger.Warn(ctx, "Timeout waiting for registry shutdown", "service", serviceName)
		}
	}
}

// UpdateStatus updates the status of the registered instance for the given service
func (r *registry) UpdateStatus(_ context.Context, serviceName execution.ServiceName, status execution.ServiceStatus) error {
	r.registrationsMu.Lock()
	defer r.registrationsMu.Unlock()

	reg, exists := r.registrations[serviceName]
	if !exists {
		return fmt.Errorf("not registered")
	}

	reg.instanceInfo.Status = status
	return writeInstanceFile(reg.fileName, reg.instanceInfo)
}

// startHeartbeat starts a background goroutine to update heartbeat for a specific service instance
// Must be called with registrationsMu held
func (r *registry) startHeartbeat(ctx context.Context, serviceName execution.ServiceName, interval time.Duration) error {
	reg := r.registrations[serviceName]
	if reg == nil {
		return fmt.Errorf("service not registered")
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	reg.cancel = cancel

	reg.wg.Add(1)
	go func() {
		defer reg.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Check if file exists, recreate if needed
				if _, err := os.Stat(reg.fileName); os.IsNotExist(err) {
					// File doesn't exist, recreate it
					if err := writeInstanceFile(reg.fileName, reg.instanceInfo); err != nil {
						logger.Error(ctx, "Failed to recreate instance file", "err", err, "file", reg.fileName)
						continue
					}
				}

				// Update modification time
				now := time.Now()
				if err := os.Chtimes(reg.fileName, now, now); err != nil {
					logger.Error(ctx, "Failed to update heartbeat", "err", err, "file", reg.fileName)
				}
			}
		}
	}()

	return nil
}
