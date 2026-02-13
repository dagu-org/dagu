package scheduler_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestReadEntries(t *testing.T) {
	t.Run("InvalidDirectory", func(t *testing.T) {
		manager := scheduler.NewEntryReader("invalid_directory", nil)
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
}
