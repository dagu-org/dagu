package fileserviceregistry

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/dirlock"
)

// finder provides file-based service registry
type finder struct {
	baseDir      string
	serviceName  execution.ServiceName
	staleTimeout time.Duration
	dirLock      dirlock.DirLock
	mu           sync.Mutex

	// Cache fields
	cachedMembers []execution.HostInfo
	cacheTime     time.Time
	cacheDuration time.Duration
}

// newFinder creates a new finder for a specific service
func newFinder(baseDir string, serviceName execution.ServiceName) *finder {
	serviceDir := filepath.Join(baseDir, string(serviceName))

	// Create directory lock for this service
	lock := dirlock.New(serviceDir, &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,       // Lock is stale after 5 seconds
		RetryInterval:  50 * time.Millisecond, // Retry every 50ms
	})

	return &finder{
		baseDir:       baseDir,
		serviceName:   serviceName,
		staleTimeout:  30 * time.Second, // Consider instances stale after 30 seconds
		dirLock:       lock,
		cacheDuration: 3 * time.Second, // Cache members for 15 seconds
	}
}

// members returns all active instances of the service
func (f *finder) members(ctx context.Context) ([]execution.HostInfo, error) {
	f.mu.Lock()

	// Check if we have a valid cache
	if len(f.cachedMembers) > 0 && time.Since(f.cacheTime) < f.cacheDuration {
		members := make([]execution.HostInfo, len(f.cachedMembers))
		copy(members, f.cachedMembers)
		f.mu.Unlock()
		return members, nil
	}
	f.mu.Unlock()

	serviceDir := filepath.Join(f.baseDir, string(f.serviceName))

	// If directory doesn't exist, return empty list (no instances)
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return []execution.HostInfo{}, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Acquire lock for reading and cleaning
	if f.dirLock != nil {
		if err := f.dirLock.TryLock(); err != nil {
			// If we can't get the lock, still try to read (another process may be cleaning)
			logger.Warn(ctx, "Could not acquire lock for service directory", "service", f.serviceName)
		} else {
			// Ensure we unlock when done
			defer func() {
				if err := f.dirLock.Unlock(); err != nil {
					logger.Error(ctx, "Failed to unlock service directory", "service", f.serviceName, "err", err)
				}
			}()
		}
	}

	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil, err
	}

	members := []execution.HostInfo{}
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
		if time.Since(fileInfo.ModTime()) > f.staleTimeout {
			staleFiles = append(staleFiles, instancePath)
			continue
		}

		info, err := readInstanceFile(instancePath)
		if err != nil {
			// Skip invalid files
			continue
		}

		members = append(members, execution.HostInfo{
			ID:        info.ID,
			Host:      info.Host,
			Port:      info.Port,
			Status:    info.Status,
			StartedAt: info.StartedAt,
		})
	}

	// Second pass: remove stale files (only if we have the lock)
	if f.dirLock != nil && len(staleFiles) > 0 {
		// We already have the lock from above, so we can safely remove stale files
		for _, staleFile := range staleFiles {
			if err := removeInstanceFile(staleFile); err != nil {
				logger.Error(ctx, "Failed to remove stale instance file", "file", staleFile, "err", err)
			}
		}
	}

	// Update cache if members is not empty
	if len(members) > 0 {
		f.cachedMembers = make([]execution.HostInfo, len(members))
		copy(f.cachedMembers, members)
		f.cacheTime = time.Now()
	}

	return members, nil
}
