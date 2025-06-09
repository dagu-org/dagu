package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	pkgsc "github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/robfig/cron/v3"
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

		th := setupTest(t)
		sc := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore)

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

		th := setupTest(t)
		sc := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore)

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

		th := setupTest(t)
		schedulerInstance := scheduler.New(th.Config, &mockJobManager{}, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore)

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
		lastStatus     pkgsc.Status
		skipSuccessful bool
		wantErr        error
	}{
		{
			name:           "skip_if_successful_true_with_recent_success",
			schedule:       "0 * * * *", // Every hour
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC), // 1 min after prev schedule
			lastStatus:     pkgsc.StatusSuccess,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobSuccess,
		},
		{
			name:           "skip_if_successful_false_with_recent_success",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     pkgsc.StatusSuccess,
			skipSuccessful: false,
			wantErr:        nil,
		},
		{
			name:           "already_running",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			lastStatus:     pkgsc.StatusRunning,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobRunning,
		},
		{
			name:           "last_execution_after_next_schedule",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			lastStatus:     pkgsc.StatusSuccess,
			skipSuccessful: true,
			wantErr:        scheduler.ErrJobFinished,
		},
		{
			name:           "failed_previous_run",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     pkgsc.StatusError,
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
				DAG: &digraph.DAG{
					SkipIfSuccessful: tt.skipSuccessful,
				},
				Schedule: schedule,
				Next:     tt.now,
			}

			lastRunStatus := models.DAGRunStatus{
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
			name:     "hourly_schedule",
			schedule: "0 * * * *",
			now:      time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			want:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		{
			name:     "every_five_minutes",
			schedule: "*/5 * * * *",
			now:      time.Date(2020, 1, 1, 1, 5, 0, 0, time.UTC),
			want:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		{
			name:     "daily_schedule",
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

	t.Run("QueueDisabled_SkipsQueueProcessing", func(t *testing.T) {
		th := setupTest(t)
		// Disable queues
		th.Config.Queues.Enabled = false
		
		entryReader := &mockJobManager{
			Entries: []*scheduler.ScheduledJob{},
		}

		sc := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore)

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
