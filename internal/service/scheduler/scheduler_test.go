// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()

	t.Run("Restart", func(t *testing.T) {
		ctx := context.Background()
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		// Parse a cron that fires at minute 0 (matches "now")
		cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		parsed, err := cronParser.Parse("0 * * * *")
		require.NoError(t, err)

		entryReader := newMockJobManager()
		entryReader.LoadedDAGs = []*core.DAG{
			{
				Name: "restart-dag",
				RestartSchedule: []core.Schedule{
					{Expression: "0 * * * *", Parsed: parsed},
				},
			},
		}

		th := test.SetupScheduler(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
		require.NoError(t, err)
		sc.SetClock(func() time.Time { return now })

		// Track restart calls via the planner's Restart function
		var restartCount atomic.Int32
		restartScheduleTimeCh := make(chan time.Time, 1)
		sc.SetRestartFunc(func(_ context.Context, _ *core.DAG, scheduleTime time.Time) error {
			restartCount.Add(1)
			select {
			case restartScheduleTimeCh <- scheduleTime:
			default:
			}
			return nil
		})

		errCh := startSchedulerAsync(t, sc, ctx)
		defer stopSchedulerAndWait(t, sc, errCh, ctx)

		require.Eventually(t, func() bool {
			return restartCount.Load() >= int32(1)
		}, 5*time.Second, 10*time.Millisecond, "restart should have been called")
		select {
		case restartScheduleTime := <-restartScheduleTimeCh:
			require.False(t, restartScheduleTime.IsZero())
		default:
			t.Fatal("restart schedule time was not recorded")
		}
	})
	t.Run("Start", func(t *testing.T) {
		ctx := context.Background()
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		parsed, err := cronParser.Parse("0 * * * *")
		require.NoError(t, err)

		entryReader := newMockJobManager()
		entryReader.LoadedDAGs = []*core.DAG{
			{
				Name: "start-dag",
				Schedule: []core.Schedule{
					{Expression: "0 * * * *", Parsed: parsed},
				},
			},
		}

		th := test.SetupScheduler(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
		require.NoError(t, err)
		sc.SetClock(func() time.Time { return now })

		var dispatchCount atomic.Int32
		sc.SetDispatchFunc(func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType, _ time.Time) error {
			dispatchCount.Add(1)
			return nil
		})

		errCh := startSchedulerAsync(t, sc, ctx)
		defer stopSchedulerAndWait(t, sc, errCh, ctx)

		require.Eventually(t, func() bool {
			return dispatchCount.Load() >= int32(1)
		}, 5*time.Second, 10*time.Millisecond, "dispatch should have been called for start schedule")
	})
	t.Run("NextTick", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)

		th := test.SetupScheduler(t)
		schedulerInstance, err := scheduler.New(th.Config, newMockJobManager(), th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
		require.NoError(t, err)

		next := schedulerInstance.NextTick(now)
		require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
	})
}

func TestFileLockPreventsMultipleInstances(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	entryReader := newMockJobManager()

	th := test.SetupScheduler(t)

	// Create first scheduler instance
	sc1, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc1.SetClock(clock)

	// Start first scheduler
	ctx := context.Background()
	errCh1 := startSchedulerAsync(t, sc1, ctx)

	// Create second scheduler instance with same config
	sc2, err := scheduler.New(th.Config, newMockJobManager(), th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc2.SetClock(clock)

	// Try to start second scheduler - should wait for lock
	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- sc2.Start(ctx)
	}()

	time.Sleep(time.Millisecond * 500)
	select {
	case err := <-errCh2:
		require.NoError(t, err)
		t.Fatal("second scheduler exited before first one stopped")
	default:
	}
	// Check if second scheduler is still not running
	require.False(t, sc2.IsRunning(), "Second scheduler should not be running while first one is active")

	// Stop first scheduler
	sc1.Stop(ctx)

	// Wait for the first scheduler to finish
	select {
	case err := <-errCh1:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("First scheduler did not stop in time")
	}

	require.False(t, sc1.IsRunning(), "First scheduler should not be running after stop")

	// Give second scheduler time to start
	requireSchedulerRunning(t, sc2, errCh2)

	// Stop second scheduler to clean up
	stopSchedulerAndWait(t, sc2, errCh2, ctx)
}

func TestScheduler_StopSchedule(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	parsed, err := cronParser.Parse("0 * * * *")
	require.NoError(t, err)

	entryReader := newMockJobManager()
	entryReader.LoadedDAGs = []*core.DAG{
		{
			Name: "stop-dag",
			StopSchedule: []core.Schedule{
				{Expression: "0 * * * *", Parsed: parsed},
			},
		},
	}

	th := test.SetupScheduler(t)
	sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc.SetClock(func() time.Time { return now })

	// GetLatestStatus must return Running for the stop guard
	sc.SetGetLatestStatusFunc(func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
		return exec.DAGRunStatus{Status: core.Running}, nil
	})

	var stopCount atomic.Int32
	sc.SetStopFunc(func(_ context.Context, _ *core.DAG) error {
		stopCount.Add(1)
		return nil
	})

	ctx := context.Background()
	errCh := startSchedulerAsync(t, sc, ctx)
	defer stopSchedulerAndWait(t, sc, errCh, ctx)

	require.Eventually(t, func() bool {
		return stopCount.Load() >= int32(1)
	}, 5*time.Second, 10*time.Millisecond, "stop function should have been called")
}

func TestScheduler_GracefulShutdown(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	parsed, err := cronParser.Parse("*/5 * * * *")
	require.NoError(t, err)

	entryReader := newMockJobManager()
	entryReader.LoadedDAGs = []*core.DAG{
		{
			Name: "shutdown-dag",
			Schedule: []core.Schedule{
				{Expression: "*/5 * * * *", Parsed: parsed},
			},
		},
	}

	th := test.SetupScheduler(t)
	sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc.SetClock(func() time.Time { return now })

	ctx := context.Background()
	errCh := startSchedulerAsync(t, sc, ctx)

	// Stop and verify it completes within 5 seconds
	done := make(chan struct{})
	go func() {
		sc.Stop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5 seconds")
	}

	require.False(t, sc.IsRunning(), "scheduler should not be running after stop")

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler Start() did not return after Stop()")
	}
}

func TestScheduler_StopWhileWaitingForLock(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	th := test.SetupScheduler(t)

	sc1, err := scheduler.New(th.Config, newMockJobManager(), th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc1.SetClock(clock)

	ctx := context.Background()
	errCh1 := startSchedulerAsync(t, sc1, ctx)
	defer stopSchedulerAndWait(t, sc1, errCh1, ctx)

	sc2, err := scheduler.New(th.Config, newMockJobManager(), th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc2.SetClock(clock)

	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- sc2.Start(ctx)
	}()

	time.Sleep(500 * time.Millisecond)
	require.False(t, sc2.IsRunning(), "Second scheduler should still be waiting on the lock")

	stopDone := make(chan struct{})
	go func() {
		sc2.Stop(ctx)
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() blocked while Start() was waiting on the scheduler lock")
	}

	select {
	case err := <-errCh2:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("waiting scheduler Start() did not return after Stop()")
	}
}

func startSchedulerAsync(t *testing.T, sc *scheduler.Scheduler, ctx context.Context) chan error {
	t.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sc.Start(ctx)
	}()

	requireSchedulerRunning(t, sc, errCh)
	return errCh
}

func requireSchedulerRunning(t *testing.T, sc *scheduler.Scheduler, errCh <-chan error) {
	t.Helper()

	var startErr error
	var exited bool
	require.Eventually(t, func() bool {
		if sc.IsRunning() {
			return true
		}
		select {
		case err := <-errCh:
			startErr = err
			exited = true
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond, "scheduler did not start in time")

	if exited {
		require.NoError(t, startErr)
		t.Fatal("scheduler exited before reporting running")
	}
}

func stopSchedulerAndWait(t *testing.T, sc *scheduler.Scheduler, errCh <-chan error, ctx context.Context) {
	t.Helper()

	sc.Stop(ctx)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler Start() did not return after Stop()")
	}
}
