// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core/spec"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestCronScheduleRunsTwice verifies that a DAG with */1 * * * * schedule
// runs twice in two minutes.
func TestCronScheduleRunsTwice(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "dagu-cron-test-*")
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
	var schedulerErr error
	var schedulerStopped bool
	pollSchedulerErr := func() error {
		if schedulerStopped {
			return schedulerErr
		}
		select {
		case err := <-errCh:
			schedulerStopped = true
			if err == nil {
				err = errors.New("scheduler exited unexpectedly before test completed")
			}
			schedulerErr = err
		default:
		}
		return schedulerErr
	}

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		if err := pollSchedulerErr(); err != nil {
			return true
		}
		return len(th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)) >= 2
	}, 2*time.Minute+30*time.Second, 5*time.Second)
	require.NoError(t, schedulerErr)

	schedulerInstance.Stop(ctx)
	cancel()

	if !schedulerStopped {
		select {
		case err = <-errCh:
			require.True(t,
				err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
				"unexpected scheduler shutdown error: %v", err,
			)
		case <-time.After(5 * time.Second):
		}
	}

	runs := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.GreaterOrEqual(t, len(runs), 2)
}
