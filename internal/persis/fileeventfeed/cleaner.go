// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileeventfeed

import (
	"log/slog"
	"sync"
	"time"
)

type cleaner struct {
	store    *Store
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newCleaner(store *Store) *cleaner {
	c := &cleaner{
		store:  store,
		stopCh: make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *cleaner) run() {
	c.cleanup()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

func (c *cleaner) stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *cleaner) cleanup() {
	if c.store == nil {
		return
	}
	if err := c.store.purgeExpiredShards(time.Now()); err != nil {
		slog.Warn("fileeventfeed: cleanup failed", slog.String("error", err.Error()))
	}
}
