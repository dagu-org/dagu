package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestReadEntries(t *testing.T) {
	expectedNext := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	now := expectedNext.Add(-time.Second)

	t.Run("InvalidDirectory", func(t *testing.T) {
		manager := scheduler.NewEntryReader("invalid_directory", nil, runtime.Manager{}, nil, "")
		jobs, err := manager.Next(context.Background(), expectedNext)
		require.NoError(t, err)
		require.Len(t, jobs, 0)
	})
	t.Run("InitAndNext", func(t *testing.T) {
		th := test.SetupScheduler(t)
		ctx := context.Background()

		err := th.EntryReader.Init(ctx)
		require.NoError(t, err)

		jobs, err := th.EntryReader.Next(ctx, now)
		require.NoError(t, err)
		require.NotEmpty(t, jobs, "jobs should not be empty")

		job := jobs[0]
		next := job.Next
		require.Equal(t, expectedNext, next)
	})
	t.Run("SuspendedJob", func(t *testing.T) {
		th := test.SetupScheduler(t)
		ctx := context.Background()

		err := th.EntryReader.Init(ctx)
		require.NoError(t, err)

		beforeSuspend, err := th.EntryReader.Next(ctx, now)
		require.NoError(t, err)

		// find the job and suspend it
		job := findJobByName(t, beforeSuspend, "scheduled_job").Job
		dagJob, ok := job.(*scheduler.DAGRunJob)
		require.True(t, ok)

		err = th.DAGStore.ToggleSuspend(ctx, dagJob.DAG.Name, true)
		require.NoError(t, err)

		// check if the job is suspended and not returned
		afterSuspend, err := th.EntryReader.Next(ctx, now)
		require.NoError(t, err)
		require.Equal(t, len(afterSuspend), len(beforeSuspend)-1, "suspended job should not be returned")
	})
}

func findJobByName(t *testing.T, jobs []*scheduler.ScheduledJob, name string) *scheduler.ScheduledJob {
	t.Helper()

	for _, job := range jobs {
		dagJob, ok := job.Job.(*scheduler.DAGRunJob)
		if ok && dagJob.DAG.Name == name {
			return job
		}
	}

	t.Fatalf("job %s not found", name)
	return nil
}
