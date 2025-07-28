package filediscovery

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/dirlock"
)

// resolver implements models.ServiceResolver for file-based discovery
type resolver struct {
	baseDir      string
	serviceName  models.ServiceName
	staleTimeout time.Duration
	dirLock      dirlock.DirLock
	mu           sync.Mutex
}

// newResolver creates a new resolver for a specific service
func newResolver(baseDir string, serviceName models.ServiceName) *resolver {
	serviceDir := filepath.Join(baseDir, string(serviceName))

	// Create directory lock for this service
	lock, _ := dirlock.New(serviceDir, &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,       // Lock is stale after 5 seconds
		RetryInterval:  50 * time.Millisecond, // Retry every 50ms
	})

	return &resolver{
		baseDir:      baseDir,
		serviceName:  serviceName,
		staleTimeout: 30 * time.Second, // Consider instances stale after 30 seconds
		dirLock:      lock,
	}
}

// Members returns all active instances of the service
func (r *resolver) Members(ctx context.Context) ([]models.HostInfo, error) {
	serviceDir := filepath.Join(r.baseDir, string(r.serviceName))

	// If directory doesn't exist, return empty list (no instances)
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return []models.HostInfo{}, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Acquire lock for reading and cleaning
	if r.dirLock != nil {
		if err := r.dirLock.TryLock(); err != nil {
			// If we can't get the lock, still try to read (another process may be cleaning)
			logger.Warn(ctx, "Could not acquire lock for service directory", "service", r.serviceName)
		} else {
			// Ensure we unlock when done
			defer func() {
				if !r.dirLock.IsHeldByMe() {
					return
				}
				if err := r.dirLock.Unlock(); err != nil {
					logger.Error(ctx, "Failed to unlock service directory", "service", r.serviceName, "err", err)
				}
			}()
		}
	}

	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil, err
	}

	members := []models.HostInfo{}
	staleFiles := []string{}

	// First pass: collect members and identify stale files
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		select {
		case <-ctx.Done():
			return members, ctx.Err()
		default:
		}

		instancePath := filepath.Join(serviceDir, entry.Name())

		// Get file info to check modification time
		fileInfo, err := os.Stat(instancePath)
		if err != nil {
			continue
		}

		// Check if instance is stale
		if time.Since(fileInfo.ModTime()) > r.staleTimeout {
			staleFiles = append(staleFiles, instancePath)
			continue
		}

		info, err := readInstanceFile(instancePath)
		if err != nil {
			// Skip invalid files
			continue
		}

		members = append(members, models.HostInfo{
			ID:       info.ID,
			HostPort: info.HostPort,
		})
	}

	// Second pass: remove stale files (only if we have the lock)
	if r.dirLock != nil && len(staleFiles) > 0 {
		// We already have the lock from above, so we can safely remove stale files
		for _, staleFile := range staleFiles {
			if err := removeInstanceFile(staleFile); err != nil {
				logger.Error(ctx, "Failed to remove stale instance file", "file", staleFile, "err", err)
			}
		}
	}

	return members, nil
}
