package fileserviceregistry

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/execution"
)

const (
	defaultStaleTimeout  = 30 * time.Second
	defaultCacheDuration = 3 * time.Second
)

// finder discovers active service instances from the file system
type finder struct {
	baseDir     string
	serviceName execution.ServiceName
	quarantine  *quarantine
	cleaner     *cleaner

	// Cache to avoid excessive file system access
	mu            sync.Mutex
	cachedMembers []execution.HostInfo
	cacheTime     time.Time
	cacheDuration time.Duration
}

// newFinder creates a finder for discovering service instances
func newFinder(baseDir string, serviceName execution.ServiceName, enableCleanup bool) *finder {
	f := &finder{
		baseDir:       baseDir,
		serviceName:   serviceName,
		quarantine:    newQuarantine(defaultStaleTimeout),
		cacheDuration: defaultCacheDuration,
	}

	if enableCleanup {
		f.cleaner = newCleaner(baseDir, serviceName)
	}

	return f
}

// close stops background cleanup if running
func (f *finder) close() {
	if f.cleaner != nil {
		f.cleaner.stop()
	}
}

// members returns all active instances of the service
func (f *finder) members(ctx context.Context) ([]execution.HostInfo, error) {
	// Try to use cached results first
	if cached := f.getCachedMembers(); cached != nil {
		return cached, nil
	}

	// Cache miss - scan file system
	members, err := f.scanServiceDirectory(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache (only caches non-empty results to keep polling for new services)
	f.updateCache(members)

	return members, nil
}

// getCachedMembers returns cached members if the cache is still valid
func (f *finder) getCachedMembers() []execution.HostInfo {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.cachedMembers) == 0 || time.Since(f.cacheTime) >= f.cacheDuration {
		return nil
	}

	members := make([]execution.HostInfo, len(f.cachedMembers))
	copy(members, f.cachedMembers)
	return members
}

// scanServiceDirectory scans the service directory for active instances
func (f *finder) scanServiceDirectory(ctx context.Context) ([]execution.HostInfo, error) {
	instanceFiles, err := f.listInstanceFiles()
	if err != nil {
		return nil, err
	}

	members := make([]execution.HostInfo, 0, len(instanceFiles))

	for _, path := range instanceFiles {
		if ctx.Err() != nil {
			return members, ctx.Err()
		}

		if info := f.processInstanceFile(ctx, path); info != nil {
			members = append(members, *info)
		}
	}

	return members, nil
}

// listInstanceFiles returns paths to all non-quarantined instance files
func (f *finder) listInstanceFiles() ([]string, error) {
	serviceDir := filepath.Join(f.baseDir, string(f.serviceName))
	pattern := filepath.Join(serviceDir, "*.json")
	return filepath.Glob(pattern)
}

// processInstanceFile checks if an instance file is valid and returns its info
func (f *finder) processInstanceFile(ctx context.Context, path string) *execution.HostInfo {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil
	}

	// Check if file is stale
	if time.Since(fileInfo.ModTime()) > defaultStaleTimeout {
		f.quarantine.markStaleFile(ctx, path, fileInfo.ModTime())
		return nil
	}

	// Parse instance file
	instance, err := readInstanceFile(path)
	if err != nil {
		return nil
	}

	return &execution.HostInfo{
		ID:        instance.ID,
		Host:      instance.Host,
		Port:      instance.Port,
		Status:    instance.Status,
		StartedAt: instance.StartedAt,
	}
}

// updateCache stores members in the cache (only caches non-empty results)
func (f *finder) updateCache(members []execution.HostInfo) {
	if len(members) == 0 {
		// Don't cache empty results - keep scanning for new services
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.cachedMembers = make([]execution.HostInfo, len(members))
	copy(f.cachedMembers, members)
	f.cacheTime = time.Now()
}
