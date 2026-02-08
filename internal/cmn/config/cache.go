package config

import "time"

// CacheMode represents the cache configuration preset
type CacheMode string

const (
	CacheModeLow    CacheMode = "low"
	CacheModeNormal CacheMode = "normal"
	CacheModeHigh   CacheMode = "high"
)

// CacheLimits contains the cache limits for each cache type
type CacheLimits struct {
	DAG       CacheEntryLimits
	DAGRun    CacheEntryLimits
	APIKey    CacheEntryLimits
	Webhook   CacheEntryLimits
	Namespace CacheEntryLimits
}

// CacheEntryLimits contains limit and TTL for a cache
type CacheEntryLimits struct {
	Limit int
	TTL   time.Duration
}

// Limits returns the cache limits for the given mode
func (m CacheMode) Limits() CacheLimits {
	switch m {
	case CacheModeLow:
		return CacheLimits{
			DAG:       CacheEntryLimits{Limit: 500, TTL: 12 * time.Hour},
			DAGRun:    CacheEntryLimits{Limit: 5000, TTL: 12 * time.Hour},
			APIKey:    CacheEntryLimits{Limit: 100, TTL: 15 * time.Minute},
			Webhook:   CacheEntryLimits{Limit: 100, TTL: 15 * time.Minute},
			Namespace: CacheEntryLimits{Limit: 50, TTL: 15 * time.Minute},
		}
	case CacheModeHigh:
		return CacheLimits{
			DAG:       CacheEntryLimits{Limit: 5000, TTL: 12 * time.Hour},
			DAGRun:    CacheEntryLimits{Limit: 50000, TTL: 12 * time.Hour},
			APIKey:    CacheEntryLimits{Limit: 1000, TTL: 15 * time.Minute},
			Webhook:   CacheEntryLimits{Limit: 1000, TTL: 15 * time.Minute},
			Namespace: CacheEntryLimits{Limit: 500, TTL: 15 * time.Minute},
		}
	default: // CacheModeNormal or any invalid value defaults to normal
		return CacheLimits{
			DAG:       CacheEntryLimits{Limit: 1000, TTL: 12 * time.Hour},
			DAGRun:    CacheEntryLimits{Limit: 10000, TTL: 12 * time.Hour},
			APIKey:    CacheEntryLimits{Limit: 500, TTL: 15 * time.Minute},
			Webhook:   CacheEntryLimits{Limit: 500, TTL: 15 * time.Minute},
			Namespace: CacheEntryLimits{Limit: 100, TTL: 15 * time.Minute},
		}
	}
}

// IsValid returns true if the cache mode is valid
func (m CacheMode) IsValid() bool {
	switch m {
	case CacheModeLow, CacheModeNormal, CacheModeHigh:
		return true
	default:
		return false
	}
}
