package metrics

import (
	"context"
	"sync/atomic"
	"time"
)

var (
	startTime time.Time
	uptime    atomic.Int64
)

// StartUptime starts the uptime counter
func StartUptime(ctx context.Context) {
	startTime = time.Now()
	go func() {
		for {
			ms := time.Since(startTime).Milliseconds()
			uptime.Store(ms / 1000)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

// GetUptime returns the current uptime in seconds
func GetUptime() int64 {
	return uptime.Load()
}
