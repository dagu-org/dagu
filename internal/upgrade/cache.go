package upgrade

import (
	"context"
	"fmt"
	"time"
)

const (
	// CacheFileName is the name of the upgrade check cache file.
	CacheFileName = "upgrade-check.json"

	// CacheTTL is how long the cache is valid.
	CacheTTL = 24 * time.Hour
)

// UpgradeCheckCache stores the result of an upgrade check.
type UpgradeCheckCache struct {
	LastCheck       time.Time `json:"lastCheck"`
	LatestVersion   string    `json:"latestVersion"`
	CurrentVersion  string    `json:"currentVersion"`
	UpdateAvailable bool      `json:"updateAvailable"`
}

// IsCacheValid checks if the cache is still valid (not expired).
func IsCacheValid(cache *UpgradeCheckCache) bool {
	if cache == nil {
		return false
	}
	return time.Since(cache.LastCheck) < CacheTTL
}

// CheckAndUpdateCache checks for updates if cache is stale and updates the cache.
// This function is designed to be called asynchronously.
func CheckAndUpdateCache(store CacheStore, currentVersion string) (*UpgradeCheckCache, error) {
	// Skip update check for dev builds
	if currentVersion == "dev" || currentVersion == "0.0.0" {
		return nil, nil
	}

	cache, _ := store.Load()

	// If cache is valid and current version matches, return cached result
	if cache != nil && IsCacheValid(cache) && cache.CurrentVersion == currentVersion {
		return cache, nil
	}

	currentV, err := ParseVersion(currentVersion)
	if err != nil {
		return nil, err
	}

	client := NewGitHubClient()
	release, err := client.GetLatestRelease(context.Background(), false)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	latestV, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version: %w", err)
	}

	newCache := &UpgradeCheckCache{
		LastCheck:       time.Now(),
		LatestVersion:   release.TagName,
		CurrentVersion:  currentVersion,
		UpdateAvailable: IsNewer(currentV, latestV),
	}

	_ = store.Save(newCache)

	return newCache, nil
}

// GetCachedUpdateInfo returns cached update information if available.
// Returns nil if no valid cache exists.
func GetCachedUpdateInfo(store CacheStore) *UpgradeCheckCache {
	cache, err := store.Load()
	if err != nil || cache == nil {
		return nil
	}

	// Only return if the cache is reasonably fresh (allow slightly stale cache for display)
	// We use a longer TTL for reading than for refresh decisions
	if time.Since(cache.LastCheck) > CacheTTL*2 {
		return nil
	}

	return cache
}
