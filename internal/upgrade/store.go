// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package upgrade

// CacheStore provides persistence for upgrade check cache data.
type CacheStore interface {
	Load() (*UpgradeCheckCache, error)
	Save(cache *UpgradeCheckCache) error
}
