// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package testutil

import (
	"context"
	"testing"
	"time"
)

// ContextRunner is a minimal interface for background services that are driven
// by a cancellable context.
type ContextRunner interface {
	Run(context.Context)
}

// StartContextRunner starts a context-driven runner and returns a shutdown
// function that cancels the context and waits for the runner to exit.
func StartContextRunner(t *testing.T, runner ContextRunner) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for runner shutdown")
		}
	}
}
