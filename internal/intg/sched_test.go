package intg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestCronScheduleRunsTwice verifies that a DAG with */1 * * * * schedule
// runs twice in two minutes.
func TestCronScheduleRunsTwice(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "boltbase-cron-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0755))

	dagContent := `name: cron-test
schedule: "*/1 * * * *"
steps:
  - name: test-step
    command: echo "hello"
`
	dagFile := filepath.Join(dagsDir, "cron-test.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0644))

	th := test.SetupScheduler(t, test.WithDAGsDir(dagsDir))
	schedulerInstance, err := th.NewSchedulerInstance(t)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(th.Context)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- schedulerInstance.Start(ctx) }()

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)) >= 2
	}, 2*time.Minute+30*time.Second, 5*time.Second)

	schedulerInstance.Stop(ctx)
	cancel()

	select {
	case err = <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
	}

	runs := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.GreaterOrEqual(t, len(runs), 2)
}
