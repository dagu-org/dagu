package fileserviceregistry

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// finder provides file-based service registry
type finder struct {
	baseDir      string
	serviceName  execution.ServiceName
	staleTimeout time.Duration
	mu           sync.Mutex
	cleanupCh    chan string

	// Cache fields
	cachedMembers []execution.HostInfo
	cacheTime     time.Time
	cacheDuration time.Duration
}

// newFinder creates a new finder for a specific service
func newFinder(baseDir string, serviceName execution.ServiceName) *finder {
	f := &finder{
		baseDir:       baseDir,
		serviceName:   serviceName,
		staleTimeout:  30 * time.Second, // Consider instances stale after 30 seconds
		cacheDuration: 3 * time.Second,  // Cache members briefly to avoid thrashing
		cleanupCh:     make(chan string, 32),
	}
	go f.cleanupLoop()
	return f
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

	var quarantineTargets []string

	f.mu.Lock()
	defer func() {
		f.mu.Unlock()
		f.enqueueCleanup(ctx, quarantineTargets)
	}()

	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil, err
	}

	members := []execution.HostInfo{}

	// First pass: collect members and identify stale files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext == ".gc" {
			quarantineTargets = append(quarantineTargets, filepath.Join(serviceDir, entry.Name()))
			continue
		}
		if ext != ".json" {
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
		modTime := fileInfo.ModTime()
		elapsedTime := time.Since(modTime)
		if elapsedTime > f.staleTimeout {
			if quarantinedPath, ok := f.quarantineStaleFile(ctx, instancePath); ok {
				quarantineTargets = append(quarantineTargets, quarantinedPath)
			}
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

	// Update cache if members is not empty
	if len(members) > 0 {
		f.cachedMembers = make([]execution.HostInfo, len(members))
		copy(f.cachedMembers, members)
		f.cacheTime = time.Now()
	}

	return members, nil
}

// quarantineStaleFile renames a stale instance file to a quarantined suffix for later cleanup.
func (f *finder) quarantineStaleFile(ctx context.Context, path string) (string, bool) {
	quarantinePath := path + ".gc"
	if err := os.Rename(path, quarantinePath); err != nil {
		// If the file vanished meanwhile it's already gone
		if !os.IsNotExist(err) {
			logger.Warn(ctx, "Failed to quarantine stale instance file", "file", path, "err", err)
		}
		return "", false
	}
	return quarantinePath, true
}

// enqueueCleanup schedules quarantined files for background deletion.
func (f *finder) enqueueCleanup(ctx context.Context, files []string) {
	if len(files) == 0 || f.cleanupCh == nil {
		return
	}

	for _, file := range files {
		select {
		case f.cleanupCh <- file:
		default:
			logger.Warn(ctx, "Cleanup queue is full; will retry later", "file", file)
		}
	}
}

// cleanupLoop removes quarantined files with retry backoff.
func (f *finder) cleanupLoop() {
	ctx := context.Background()
	policy := backoff.NewExponentialBackoffPolicy(200 * time.Millisecond)
	policy.MaxInterval = 2 * time.Second
	policy.MaxRetries = 5

	for file := range f.cleanupCh {
		err := backoff.Retry(ctx, func(ctx context.Context) error {
			return removeInstanceFile(file)
		}, policy, nil)
		if err != nil {
			logger.Warn(ctx, "Failed to cleanup quarantined instance file", "file", file, "err", err)
		}
	}
}
