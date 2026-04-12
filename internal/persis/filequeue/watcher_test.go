// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filequeue_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/persis/filequeue"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueueWatcher_StopAfterContextCancel(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	store := filequeue.New(th.Config.Paths.QueueDir)

	ctx, cancel := context.WithCancel(th.Context)
	defer cancel()

	watcher := store.QueueWatcher(ctx)
	_, err := watcher.Start(ctx)
	require.NoError(t, err)

	cancel()

	done := make(chan struct{})
	go func() {
		watcher.Stop(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("queue watcher Stop blocked after context cancellation")
	}
}
