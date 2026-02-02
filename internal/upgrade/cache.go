package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// GetCacheDir returns the directory for storing the cache file.
func GetCacheDir() (string, error) {
	// Use XDG config directory or fallback to ~/.config/dagu
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config")
	}

	cacheDir := filepath.Join(configDir, "dagu")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return cacheDir, nil
}

// GetCachePath returns the full path to the cache file.
func GetCachePath() (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, CacheFileName), nil
}

// LoadCache loads the upgrade check cache from disk.
func LoadCache() (*UpgradeCheckCache, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache UpgradeCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Invalid cache file, treat as no cache
		return nil, nil
	}

	return &cache, nil
}

// SaveCache saves the upgrade check cache to disk.
func SaveCache(cache *UpgradeCheckCache) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
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
func CheckAndUpdateCache(currentVersion string) (*UpgradeCheckCache, error) {
	cache, err := LoadCache()
	if err != nil {
		// Log but don't fail
		cache = nil
	}

	// If cache is valid and current version matches, return cached result
	if cache != nil && IsCacheValid(cache) && cache.CurrentVersion == currentVersion {
		return cache, nil
	}

	// Parse current version
	currentV, err := ParseVersion(currentVersion)
	if err != nil {
		// Development version or invalid - can't check for updates
		return nil, err
	}

	// Fetch latest release
	client := NewGitHubClient()
	release, err := client.GetLatestRelease(context.Background(), false)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	// Parse latest version
	latestV, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version: %w", err)
	}

	// Create new cache
	newCache := &UpgradeCheckCache{
		LastCheck:       time.Now(),
		LatestVersion:   release.TagName,
		CurrentVersion:  currentVersion,
		UpdateAvailable: IsNewer(currentV, latestV),
	}

	// Save cache (ignore errors)
	_ = SaveCache(newCache)

	return newCache, nil
}

// GetCachedUpdateInfo returns cached update information if available.
// Returns nil if no valid cache exists.
func GetCachedUpdateInfo() *UpgradeCheckCache {
	cache, err := LoadCache()
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
