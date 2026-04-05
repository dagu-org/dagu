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
	"github.com/dagu-org/dagu/internal/persis/filedistributed"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetQueueFiltersDistributedRunsByLeaseFreshness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	leaseStore := filedistributed.NewDAGRunLeaseStore(filepath.Join(tmpDir, "distributed"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	createDistributedQueueRun(t, ctx, dagRunStore, leaseStore, "lease-q", "fresh-run", "lease-q", time.Now().Add(-time.Second))
	createDistributedQueueRun(t, ctx, dagRunStore, leaseStore, "lease-q", "stale-run", "lease-q", time.Now().Add(-10*time.Second))

	a := &API{
		dagRunStore:         dagRunStore,
		dagRunLeaseStore:    leaseStore,
		procStore:           procStore,
		config:              &config.Config{},
		leaseStaleThreshold: 5 * time.Second,
	}

	resp, err := a.GetQueue(ctx, openapiv1.GetQueueRequestObject{
		Name: "lease-q",
	})
	require.NoError(t, err)

	queueResp, ok := resp.(openapiv1.GetQueue200JSONResponse)
	require.True(t, ok)
	require.Len(t, queueResp.Running, 1)
	assert.Equal(t, "fresh-run", queueResp.Running[0].DagRunId)
	assert.Equal(t, openapiv1.StatusRunning, queueResp.Running[0].Status)
}

func TestGetQueueFallsBackToDAGNameWhenLeaseQueueIsEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	leaseStore := filedistributed.NewDAGRunLeaseStore(filepath.Join(tmpDir, "distributed"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	createDistributedQueueRun(t, ctx, dagRunStore, leaseStore, "fallback-q", "fresh-run", "", time.Now().Add(-time.Second))

	a := &API{
		dagRunStore:         dagRunStore,
		dagRunLeaseStore:    leaseStore,
		procStore:           procStore,
		config:              &config.Config{},
		leaseStaleThreshold: 5 * time.Second,
	}

	resp, err := a.GetQueue(ctx, openapiv1.GetQueueRequestObject{
		Name: "fallback-q",
	})
	require.NoError(t, err)

	queueResp, ok := resp.(openapiv1.GetQueue200JSONResponse)
	require.True(t, ok)
	require.Len(t, queueResp.Running, 1)
	assert.Equal(t, "fresh-run", queueResp.Running[0].DagRunId)
}

func TestListQueueItemsUsesCursorPaginationAndSkipsRunningEntries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	queueStore := filequeue.New(filepath.Join(tmpDir, "queue"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	createQueuedQueueRun(t, ctx, dagRunStore, queueStore, "cursor-q", "run-1", core.Queued)
	createQueuedQueueRun(t, ctx, dagRunStore, queueStore, "cursor-q", "run-2", core.Running)
	createQueuedQueueRun(t, ctx, dagRunStore, queueStore, "cursor-q", "run-3", core.Queued)
	createQueuedQueueRun(t, ctx, dagRunStore, queueStore, "cursor-q", "run-4", core.Queued)

	a := &API{
		dagRunStore: dagRunStore,
		queueStore:  queueStore,
		procStore:   procStore,
		config:      &config.Config{},
	}

	firstResp, err := a.ListQueueItems(ctx, openapiv1.ListQueueItemsRequestObject{
		Name: "cursor-q",
		Params: openapiv1.ListQueueItemsParams{
			Limit: queueListLimitPtr(2),
		},
	})
	require.NoError(t, err)

	firstPage, ok := firstResp.(openapiv1.ListQueueItems200JSONResponse)
	require.True(t, ok)
	require.Len(t, firstPage.Items, 2)
	require.NotNil(t, firstPage.NextCursor)
	assert.Equal(t, "run-1", firstPage.Items[0].DagRunId)
	assert.Equal(t, "run-3", firstPage.Items[1].DagRunId)

	secondResp, err := a.ListQueueItems(ctx, openapiv1.ListQueueItemsRequestObject{
		Name: "cursor-q",
		Params: openapiv1.ListQueueItemsParams{
			Limit:  queueListLimitPtr(2),
			Cursor: firstPage.NextCursor,
		},
	})
	require.NoError(t, err)

	secondPage, ok := secondResp.(openapiv1.ListQueueItems200JSONResponse)
	require.True(t, ok)
	require.Len(t, secondPage.Items, 1)
	assert.Equal(t, "run-4", secondPage.Items[0].DagRunId)
	assert.Nil(t, secondPage.NextCursor)
}

func createDistributedQueueRun(
	t *testing.T,
	ctx context.Context,
	store exec.DAGRunStore,
	leaseStore exec.DAGRunLeaseStore,
	name string,
	dagRunID string,
	leaseQueueName string,
	lastHeartbeatAt time.Time,
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
	status.AttemptID = attempt.ID()
	status.ProcGroup = name
	status.WorkerID = "worker-1"
	status.StartedAt = time.Now().UTC().Format(time.RFC3339)
	status.CreatedAt = time.Now().UnixMilli()

	require.NoError(t, attempt.Write(ctx, status))
	require.NoError(t, leaseStore.Upsert(ctx, exec.DAGRunLease{
		AttemptKey:      exec.GenerateAttemptKey(name, dagRunID, name, dagRunID, attempt.ID()),
		DAGRun:          exec.NewDAGRunRef(name, dagRunID),
		Root:            exec.NewDAGRunRef(name, dagRunID),
		AttemptID:       attempt.ID(),
		QueueName:       leaseQueueName,
		WorkerID:        "worker-1",
		LastHeartbeatAt: lastHeartbeatAt.UTC().UnixMilli(),
	}))
}

func createQueuedQueueRun(
	t *testing.T,
	ctx context.Context,
	store exec.DAGRunStore,
	queueStore exec.QueueStore,
	name string,
	dagRunID string,
	status core.Status,
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

	runStatus := exec.InitialStatus(dag)
	runStatus.Status = status
	runStatus.DAGRunID = dagRunID
	runStatus.AttemptID = attempt.ID()
	runStatus.ProcGroup = name
	runStatus.QueuedAt = time.Now().UTC().Format(time.RFC3339)
	runStatus.CreatedAt = time.Now().UnixMilli()
	if status == core.Running {
		runStatus.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}

	require.NoError(t, attempt.Write(ctx, runStatus))
	require.NoError(t, queueStore.Enqueue(ctx, name, exec.QueuePriorityLow, exec.NewDAGRunRef(name, dagRunID)))
}

func queueListLimitPtr(v int) *openapiv1.QueueListLimit {
	limit := openapiv1.QueueListLimit(v)
	return &limit
}
