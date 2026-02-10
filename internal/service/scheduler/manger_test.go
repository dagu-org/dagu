package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestReadEntries(t *testing.T) {
	t.Run("InvalidDirectory", func(t *testing.T) {
		manager := scheduler.NewEntryReader("invalid_directory", nil, test.SetupScheduler(t).DAGRunMgr, nil, "")
		err := manager.Init(context.Background())
		require.Error(t, err)
	})
	t.Run("InitAndDAGs", func(t *testing.T) {
		th := test.SetupScheduler(t)
		ctx := context.Background()

		err := th.EntryReader.Init(ctx)
		require.NoError(t, err)

		dags := th.EntryReader.DAGs()
		require.NotEmpty(t, dags, "DAGs should not be empty")
	})
	t.Run("EventsChannel", func(t *testing.T) {
		th := test.SetupScheduler(t)
		ctx := context.Background()

		err := th.EntryReader.Init(ctx)
		require.NoError(t, err)

		ch := th.EntryReader.Events()
		require.NotNil(t, ch, "Events channel should not be nil")
	})
	t.Run("StopRestartJobs", func(t *testing.T) {
		th := test.SetupScheduler(t)
		ctx := context.Background()

		err := th.EntryReader.Init(ctx)
		require.NoError(t, err)

		// StopRestartJobs should return only stop/restart jobs (not start)
		jobs := th.EntryReader.StopRestartJobs(ctx, time.Now())
		// The test DAGs may or may not have stop/restart schedules
		// but the method should not panic
		_ = jobs
	})
}
