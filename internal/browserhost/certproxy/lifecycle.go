// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"context"
	"errors"
	"time"

	"github.com/dagucloud/dagu/v2/internal/browserhost"
)

const (
	// browserPollInterval spaces the checks that the browser still runs.
	browserPollInterval = 2 * time.Second
	// defaultGrace is how long the proxy waits for the browser to answer
	// for the first time when Config sets no grace period.
	defaultGrace = 5 * time.Minute
)

// watchBrowser returns once the browser at cdpURL is gone: nothing accepts
// connections there, or another browser answers in its place. It also
// returns when no browser answers within grace, or when ctx ends. A browser
// that is slow to answer counts as running.
func watchBrowser(ctx context.Context, cdpURL string, grace, interval time.Duration) {
	deadline := time.Now().Add(grace)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var pinned string
	for {
		id, err := browserhost.BrowserID(ctx, cdpURL)
		switch {
		case ctx.Err() != nil:
			return
		case pinned == "" && err == nil:
			pinned = id
		case pinned == "" && time.Now().After(deadline):
			return
		case pinned != "" && errors.Is(err, browserhost.ErrUnreachable):
			return
		case pinned != "" && err == nil && id != pinned:
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
