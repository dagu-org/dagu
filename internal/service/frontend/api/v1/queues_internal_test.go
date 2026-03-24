// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	openapiv1 "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListQueueItemsRunningFiltersDistributedRunsByLeaseFreshness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	createDistributedQueueRun(t, ctx, dagRunStore, "lease-q", "fresh-run", time.Now().Add(-time.Second))
	createDistributedQueueRun(t, ctx, dagRunStore, "lease-q", "stale-run", time.Now().Add(-10*time.Second))

	a := &API{
		dagRunStore:         dagRunStore,
		procStore:           procStore,
		config:              &config.Config{},
		leaseStaleThreshold: 5 * time.Second,
	}

	itemType := openapiv1.ListQueueItemsParamsTypeRunning
	resp, err := a.ListQueueItems(ctx, openapiv1.ListQueueItemsRequestObject{
		Name: "lease-q",
		Params: openapiv1.ListQueueItemsParams{
			Type: &itemType,
		},
	})
	require.NoError(t, err)

	listResp, ok := resp.(openapiv1.ListQueueItems200JSONResponse)
	require.True(t, ok)
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, "fresh-run", listResp.Items[0].DagRunId)
	assert.Equal(t, openapiv1.StatusRunning, listResp.Items[0].Status)
}

func createDistributedQueueRun(
	t *testing.T,
	ctx context.Context,
	store exec.DAGRunStore,
	name string,
	dagRunID string,
	leaseAt time.Time,
) {
	t.Helper()

	dag := &core.DAG{
		Name: name,
		Steps: []core.Step{
			{Name: "step", Command: "echo hello"},
		},
	}

	attempt, err := store.CreateAttempt(ctx, dag, time.Now().UTC(), dagRunID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, attempt.Open(ctx))
	defer func() {
		require.NoError(t, attempt.Close(ctx))
	}()

	status := exec.InitialStatus(dag)
	status.Status = core.Running
	status.DAGRunID = dagRunID
	status.ProcGroup = name
	status.WorkerID = "worker-1"
	status.StartedAt = time.Now().UTC().Format(time.RFC3339)
	status.CreatedAt = time.Now().UnixMilli()
	status.LeaseAt = leaseAt.UnixMilli()

	require.NoError(t, attempt.Write(ctx, status))
}
