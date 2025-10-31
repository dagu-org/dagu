package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()

	t.Run("Start", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		scheduler.SetFixedTime(now)

		entryReader := &mockJobManager{
			Entries: []*scheduler.ScheduledJob{
				{Job: &mockJob{}, Next: now},
				{Job: &mockJob{}, Next: now.Add(time.Minute)},
			},
		}

		th := test.SetupScheduler(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
		require.NoError(t, err)

		ctx := context.Background()
		go func() {
			_ = sc.Start(ctx)
		}()

		time.Sleep(time.Second + time.Millisecond*100)
		sc.Stop(ctx)

		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RunCount.Load())
		require.Equal(t, int32(0), entryReader.Entries[1].Job.(*mockJob).RunCount.Load())
	})

	t.Run("Restart", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		scheduler.SetFixedTime(now)

		entryReader := &mockJobManager{
			Entries: []*scheduler.ScheduledJob{
				{Type: scheduler.ScheduleTypeRestart, Job: &mockJob{}, Next: now},
			},
		}

		th := test.SetupScheduler(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
		require.NoError(t, err)

		go func() {
			_ = sc.Start(context.Background())
		}()
		defer sc.Stop(context.Background())

		time.Sleep(time.Second + time.Millisecond*100)
		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RestartCount.Load())

	})
	t.Run("NextTick", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
		scheduler.SetFixedTime(now)

		th := test.SetupScheduler(t)
		schedulerInstance, err := scheduler.New(th.Config, &mockJobManager{}, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
		require.NoError(t, err)

		next := schedulerInstance.NextTick(now)
		require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
	})
}

func TestFixedTime(t *testing.T) {
	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	scheduler.SetFixedTime(fixedTime)
	require.Equal(t, fixedTime, scheduler.Now())

	// Reset
	scheduler.SetFixedTime(time.Time{})
	require.NotEqual(t, fixedTime, scheduler.Now())
}

func TestJobReady(t *testing.T) {
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	tests := []struct {
		name           string
		schedule       string
		now            time.Time
		lastRunTime    time.Time
		lastStatus     core.Status
		skipSuccessful bool
		wantErr        error
	}{
		{
			name:           "SkipIfSuccessfulTrueWithRecentSuccess",
			schedule:       "0 * * * *", // Every hour
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC), // 1 min after prev schedule
			lastStatus:     core.Succeeded,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobSuccess,
		},
		{
			name:           "SkipIfSuccessfulFalseWithRecentSuccess",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     core.Succeeded,
			skipSuccessful: false,
			wantErr:        nil,
		},
		{
			name:           "AlreadyRunning",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			lastStatus:     core.Running,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobRunning,
		},
		{
			name:           "LastExecutionAfterNextSchedule",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			lastStatus:     core.Succeeded,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobFinished,
		},
		{
			name:           "FailedPreviousRun",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     core.Failed,
			skipSuccessful: true,
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := cronParser.Parse(tt.schedule)
			require.NoError(t, err)

			scheduler.SetFixedTime(tt.now)

			job := &scheduler.DAGRunJob{
				DAG: &core.DAG{
					SkipIfSuccessful: tt.skipSuccessful,
				},
				Schedule: schedule,
				Next:     tt.now,
			}

			lastRunStatus := execution.DAGRunStatus{
				Status:    tt.lastStatus,
				StartedAt: stringutil.FormatTime(tt.lastRunTime),
			}

			err = job.Ready(context.Background(), lastRunStatus)
			require.Equal(t, tt.wantErr, err)
		})
	}
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
			got := job.PrevExecTime(context.Background())
			require.Equal(t, tt.want, got)
		})
	}
}

func TestScheduler_QueueDisabled(t *testing.T) {
	t.Parallel()

	t.Run("QueueDisabledSkipsQueueProcessing", func(t *testing.T) {
		th := test.SetupScheduler(t)
		// Disable queues
		th.Config.Queues.Enabled = false

		entryReader := &mockJobManager{
			Entries: []*scheduler.ScheduledJob{},
		}

		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
		require.NoError(t, err)

		ctx := context.Background()
		go func() {
			_ = sc.Start(ctx)
		}()

		// Give it a moment to start
		time.Sleep(time.Millisecond * 100)
		sc.Stop(ctx)

		// Test passes if no panics occur when queues are disabled
		require.True(t, true)
	})
}

func TestFileLockPreventsMultipleInstances(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	scheduler.SetFixedTime(now)

	entryReader := &mockJobManager{
		Entries: []*scheduler.ScheduledJob{},
	}

	th := test.SetupScheduler(t)

	// Create first scheduler instance
	sc1, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
	require.NoError(t, err)

	// Start first scheduler
	ctx := context.Background()
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- sc1.Start(ctx)
	}()

	// Give first scheduler time to acquire lock
	time.Sleep(time.Millisecond * 100)

	// Create second scheduler instance with same config
	sc2, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, th.ServiceRegistry, th.CoordinatorCli)
	require.NoError(t, err)

	// Try to start second scheduler - should wait for lock
	go func() {
		err := sc2.Start(ctx)
		assert.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 500)
	// Check if second scheduler is still not running
	require.False(t, sc2.IsRunning(), "Second scheduler should not be running while first one is active")

	// Stop first scheduler
	sc1.Stop(ctx)

	// Wait for the second scheduler start
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
}
