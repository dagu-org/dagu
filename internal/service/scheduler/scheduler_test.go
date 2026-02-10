package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()

	t.Run("Restart", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		entryReader := newMockJobManager()
		entryReader.StopRestartEntries = []*scheduler.ScheduledJob{
			{Type: scheduler.ScheduleTypeRestart, Job: &mockJob{}, Next: now},
		}

		th := test.SetupScheduler(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli, nil)
		require.NoError(t, err)
		sc.SetClock(func() time.Time { return now })

		go func() {
			_ = sc.Start(context.Background())
		}()
		defer sc.Stop(context.Background())

		time.Sleep(time.Second + time.Millisecond*100)
		require.Equal(t, int32(1), entryReader.StopRestartEntries[0].Job.(*mockJob).RestartCount.Load())

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

func TestPrevExecTime(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		now      time.Time
		want     time.Time
	}{
		{
			name:     "HourlySchedule",
			schedule: "0 * * * *",
			now:      time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			want:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		{
			name:     "EveryFiveMinutes",
			schedule: "*/5 * * * *",
			now:      time.Date(2020, 1, 1, 1, 5, 0, 0, time.UTC),
			want:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		{
			name:     "DailySchedule",
			schedule: "0 0 * * *",
			now:      time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			schedule, err := cronParser.Parse(tt.schedule)
			require.NoError(t, err)

			job := &scheduler.DAGRunJob{Schedule: schedule, Next: tt.now}
			got := job.PrevExecTime()
			require.Equal(t, tt.want, got)
		})
	}
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

	// Wait for the first scheduler start
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
