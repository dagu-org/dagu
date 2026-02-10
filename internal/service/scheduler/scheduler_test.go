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
		sc.SetRestartFunc(func(_ context.Context, _ *core.DAG) error {
			restartCount.Add(1)
			return nil
		})

		go func() {
			_ = sc.Start(context.Background())
		}()
		defer sc.Stop(context.Background())

		time.Sleep(time.Second + time.Millisecond*100)
		require.GreaterOrEqual(t, restartCount.Load(), int32(1))
	})
	t.Run("Start", func(t *testing.T) {
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
		sc.SetDispatchFunc(func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			dispatchCount.Add(1)
			return nil
		})

		go func() {
			_ = sc.Start(context.Background())
		}()
		defer sc.Stop(context.Background())

		time.Sleep(time.Second + time.Millisecond*100)
		require.GreaterOrEqual(t, dispatchCount.Load(), int32(1), "dispatch should have been called for start schedule")
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
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- sc1.Start(ctx)
	}()

	// Give first scheduler time to acquire lock
	time.Sleep(time.Millisecond * 100)

	// Create second scheduler instance with same config
	sc2, err := scheduler.New(th.Config, newMockJobManager(), th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
	require.NoError(t, err)
	sc2.SetClock(clock)

	// Try to start second scheduler - should wait for lock
	go func() {
		_ = sc2.Start(ctx)
	}()

	time.Sleep(time.Millisecond * 500)
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
	time.Sleep(time.Millisecond * 100)

	// Check if second scheduler is running
	require.True(t, sc2.IsRunning(), "Second scheduler should be running after first one stopped")

	// Stop second scheduler to clean up
	sc2.Stop(ctx)
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

	go func() {
		_ = sc.Start(context.Background())
	}()
	defer sc.Stop(context.Background())

	time.Sleep(time.Second + time.Millisecond*100)
	require.GreaterOrEqual(t, stopCount.Load(), int32(1), "stop function should have been called")
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- sc.Start(context.Background())
	}()

	// Wait until scheduler is running
	deadline := time.After(5 * time.Second)
	for !sc.IsRunning() {
		select {
		case <-deadline:
			t.Fatal("scheduler did not start in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Stop and verify it completes within 5 seconds
	done := make(chan struct{})
	go func() {
		sc.Stop(context.Background())
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
