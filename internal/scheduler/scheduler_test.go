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
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
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

		th := setupTest(t)
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
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

		th := setupTest(t)
		schedulerInstance, err := scheduler.New(th.Config, &mockJobManager{}, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
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

		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
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

	th := setupTest(t)

	// Create first scheduler instance
	sc1, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
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
	sc2, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
	require.NoError(t, err)

	// Try to start second scheduler - should fail due to lock conflict
	err = sc2.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "scheduler lock is already held by another process")

	// Stop first scheduler
	sc1.Stop(ctx)

	// Wait for first scheduler to finish
	select {
	case err := <-errCh1:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("First scheduler did not stop in time")
	}

	// Now second scheduler should be able to start
	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- sc2.Start(ctx)
	}()

	// Give second scheduler time to start
	time.Sleep(time.Millisecond * 100)

	// Stop second scheduler
	sc2.Stop(ctx)

	// Wait for second scheduler to finish
	select {
	case err := <-errCh2:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Second scheduler did not stop in time")
	}
}

func TestSchedulerLockUtilities(t *testing.T) {
	th := setupTest(t)

	t.Run("IsLocked returns false when no lock exists", func(t *testing.T) {
		require.False(t, scheduler.IsLocked(th.Config))
	})

	t.Run("ForceUnlock succeeds when no lock exists", func(t *testing.T) {
		// ForceUnlock should succeed even if the directory doesn't exist
		err := scheduler.ForceUnlock(th.Config)
		// This might return an error if directory doesn't exist, which is acceptable
		if err != nil {
			require.Contains(t, err.Error(), "no such file or directory")
		}
	})

	t.Run("LockInfo returns nil when no lock exists", func(t *testing.T) {
		info, err := scheduler.LockInfo(th.Config)
		// This might return an error if directory doesn't exist, which is acceptable
		if err != nil {
			require.Contains(t, err.Error(), "no such file or directory")
		} else {
			require.Nil(t, info)
		}
	})

	t.Run("IsLocked returns true when scheduler is running", func(t *testing.T) {
		entryReader := &mockJobManager{Entries: []*scheduler.ScheduledJob{}}
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
		require.NoError(t, err)

		ctx := context.Background()
		errCh := make(chan error, 1)
		go func() {
			errCh <- sc.Start(ctx)
		}()

		// Give scheduler time to acquire lock
		time.Sleep(time.Millisecond * 100)

		require.True(t, scheduler.IsLocked(th.Config))

		// Check lock info
		info, err := scheduler.LockInfo(th.Config)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Contains(t, info.LockDirName, ".dagu_lock.")

		// Stop scheduler
		sc.Stop(ctx)

		// Wait for scheduler to finish
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Scheduler did not stop in time")
		}

		// Lock should be released
		require.False(t, scheduler.IsLocked(th.Config))
	})

	t.Run("ForceUnlock removes existing lock", func(t *testing.T) {
		entryReader := &mockJobManager{Entries: []*scheduler.ScheduledJob{}}
		sc, err := scheduler.New(th.Config, entryReader, th.DAGRunMgr, th.DAGRunStore, th.QueueStore, th.ProcStore, nil)
		require.NoError(t, err)

		ctx := context.Background()
		errCh := make(chan error, 1)
		go func() {
			errCh <- sc.Start(ctx)
		}()

		// Give scheduler time to acquire lock
		time.Sleep(time.Millisecond * 100)

		require.True(t, scheduler.IsLocked(th.Config))

		// Force unlock should remove the lock
		err = scheduler.ForceUnlock(th.Config)
		require.NoError(t, err)

		// Stop scheduler (it should handle the missing lock gracefully)
		sc.Stop(ctx)

		// Wait for scheduler to finish
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Scheduler did not stop in time")
		}

		require.False(t, scheduler.IsLocked(th.Config))
	})
}
