// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	openapiv1 "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
)

type retryCoordinatorRecorder struct {
	stubCoordinatorClient
	dispatched  []*coordinatorv1.Task
	dispatchErr error
}

var _ coordinator.Client = (*retryCoordinatorRecorder)(nil)

func (c *retryCoordinatorRecorder) Dispatch(_ context.Context, task *coordinatorv1.Task) error {
	c.dispatched = append(c.dispatched, task)
	return c.dispatchErr
}

func TestRetryDAGRun_DispatchesRetryToCoordinator(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	dagFile := filepath.Join(tmpDir, "distributed-retry.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(`
name: distributed_retry_dag
worker_selector:
  region: apac
steps:
  - name: main
    command: echo distributed retry
`), 0o600))

	dag, err := spec.Load(ctx, dagFile)
	require.NoError(t, err)

	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	attempt, err := dagRunStore.CreateAttempt(
		ctx,
		dag,
		time.Now().Add(-2*time.Minute),
		"distributed-run",
		exec.NewDAGRunAttemptOptions{},
	)
	require.NoError(t, err)

	status := transform.NewStatusBuilder(dag).Create(
		"distributed-run",
		core.Failed,
		0,
		time.Now().Add(-2*time.Minute),
		transform.WithAttemptID(attempt.ID()),
		transform.WithFinishedAt(time.Now().Add(-time.Minute)),
		transform.WithError("step failed"),
	)
	require.NotEmpty(t, status.Nodes)
	status.Nodes[0].Status = core.NodeFailed
	status.Nodes[0].Error = "step failed"
	status.Nodes[0].FinishedAt = exec.FormatTime(time.Now().Add(-time.Minute))

	require.NoError(t, attempt.Open(ctx))
	require.NoError(t, attempt.Write(ctx, status))
	require.NoError(t, attempt.Close(ctx))

	coordinatorCli := &retryCoordinatorRecorder{}
	api := &API{
		dagRunStore: dagRunStore,
		config: &config.Config{
			Server: config.Server{
				Permissions: map[config.Permission]bool{
					config.PermissionRunDAGs: true,
				},
			},
		},
		coordinatorCli:  coordinatorCli,
		defaultExecMode: config.ExecutionModeLocal,
	}

	resp, err := api.RetryDAGRun(ctx, openapiv1.RetryDAGRunRequestObject{
		Name:     dag.Name,
		DagRunId: "distributed-run",
		Body: &openapiv1.RetryDAGRunJSONRequestBody{
			DagRunId: "distributed-run",
		},
	})
	require.NoError(t, err)
	_, ok := resp.(openapiv1.RetryDAGRun200Response)
	require.True(t, ok)

	require.Len(t, coordinatorCli.dispatched, 1)
	task := coordinatorCli.dispatched[0]
	require.Equal(t, coordinatorv1.Operation_OPERATION_RETRY, task.Operation)
	require.Equal(t, dag.Name, task.Target)
	require.Equal(t, "distributed-run", task.DagRunId)
	require.Equal(t, dag.WorkerSelector, task.WorkerSelector)
	require.NotNil(t, task.PreviousStatus)
}
