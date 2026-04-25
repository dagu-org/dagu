// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitForTickSignalStopsScheduler(t *testing.T) {
	t.Parallel()

	sc := &Scheduler{
		entryReader:    &staticEntryReader{},
		quit:           make(chan any),
		queueProcessor: NewQueueProcessor(nil, nil, nil, nil, config.Queues{}),
		planner:        &TickPlanner{},
	}

	sig := make(chan os.Signal, 1)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	sig <- syscall.SIGTERM
	require.False(t, sc.waitForTick(context.Background(), sig, timer))

	select {
	case <-sc.quit:
	default:
		t.Fatal("expected scheduler quit channel to close on signal")
	}
}
