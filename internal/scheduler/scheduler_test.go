package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()
	t.Run("Start", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		setFixedTime(now)

		entryReader := &mockJobManager{
			Entries: []*ScheduledJob{
				{Job: &mockJob{}, Next: now},
				{Job: &mockJob{}, Next: now.Add(time.Minute)},
			},
		}

		th := setupTest(t)
		scheduler := New(th.config, entryReader)

		go func() {
			_ = scheduler.Start(context.Background())
		}()

		time.Sleep(time.Second + time.Millisecond*100)
		scheduler.Stop(context.Background())

		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RunCount.Load())
		require.Equal(t, int32(0), entryReader.Entries[1].Job.(*mockJob).RunCount.Load())
	})
	t.Run("Restart", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		setFixedTime(now)

		entryReader := &mockJobManager{
			Entries: []*ScheduledJob{
				{Type: ScheduleTypeRestart, Job: &mockJob{}, Next: now},
			},
		}

		th := setupTest(t)
		scheduler := New(th.config, entryReader)

		go func() {
			_ = scheduler.Start(context.Background())
		}()
		defer scheduler.Stop(context.Background())

		time.Sleep(time.Second + time.Millisecond*100)
		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RestartCount.Load())
	})
	t.Run("NextTick", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
		setFixedTime(now)

		th := setupTest(t)
		schedulerInstance := New(th.config, &mockJobManager{})

		next := schedulerInstance.nextTick(now)
		require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
	})
}

func TestFixedTime(t *testing.T) {
	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	setFixedTime(fixedTime)
	require.Equal(t, fixedTime, now())

	// Reset
	setFixedTime(time.Time{})
	require.NotEqual(t, fixedTime, now())
}

func TestJobReady(t *testing.T) {
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	tests := []struct {
		name           string
		schedule       string
		now            time.Time
		lastRunTime    time.Time
		lastStatus     scheduler.Status
		skipSuccessful bool
		wantErr        error
	}{
		{
			name:           "skip_if_successful_true_with_recent_success",
			schedule:       "0 * * * *", // Every hour
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC), // 1 min after prev schedule
			lastStatus:     scheduler.StatusSuccess,
			skipSuccessful: true,
			wantErr:        ErrJobSuccess,
		},
		{
			name:           "skip_if_successful_false_with_recent_success",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     scheduler.StatusSuccess,
			skipSuccessful: false,
			wantErr:        nil,
		},
		{
			name:           "already_running",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			lastStatus:     scheduler.StatusRunning,
			skipSuccessful: true,
			wantErr:        ErrJobRunning,
		},
		{
			name:           "last_run_after_next_schedule",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			lastStatus:     scheduler.StatusSuccess,
			skipSuccessful: true,
			wantErr:        ErrJobFinished,
		},
		{
			name:           "failed_previous_run",
			schedule:       "0 * * * *",
			now:            time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			lastRunTime:    time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
			lastStatus:     scheduler.StatusError,
			skipSuccessful: true,
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := cronParser.Parse(tt.schedule)
			require.NoError(t, err)

			setFixedTime(tt.now)

			job := &dagJob{
				DAG: &digraph.DAG{
					SkipIfSuccessful: tt.skipSuccessful,
				},
				Schedule: schedule,
				Next:     tt.now,
			}

			lastRunStatus := persistence.Status{
				Status:    tt.lastStatus,
				StartedAt: stringutil.FormatTime(tt.lastRunTime),
			}

			err = job.ready(context.Background(), lastRunStatus)
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

			job := &dagJob{Schedule: schedule, Next: tt.now}
			got := job.prevExecTime(context.Background())
			require.Equal(t, tt.want, got)
		})
	}
}
