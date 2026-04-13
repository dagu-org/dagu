// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package metrics

import (
	"context"
	"sync/atomic"
	"time"
)

var (
	startUnixNano atomic.Int64
	uptime        atomic.Int64
)

// StartUptime starts the uptime counter
func StartUptime(ctx context.Context) {
	startedAt := time.Now()
	startUnixNano.Store(startedAt.UnixNano())
	uptime.Store(0)

	go func(start time.Time) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			ms := time.Since(start).Milliseconds()
			uptime.Store(ms / 1000)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}(startedAt)
}

// GetUptime returns the current uptime in seconds
func GetUptime() int64 {
	return uptime.Load()
}
