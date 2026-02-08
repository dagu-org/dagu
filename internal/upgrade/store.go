package upgrade

// CacheStore provides persistence for upgrade check cache data.
type CacheStore interface {
	Load() (*UpgradeCheckCache, error)
	Save(cache *UpgradeCheckCache) error
}
