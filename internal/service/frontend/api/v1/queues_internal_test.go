// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"os"
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

func TestListQueueItemsRunningFiltersDistributedRunsByLeaseFreshness(t *testing.T) {
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

func TestListQueueItemsRunningFallsBackToDAGNameWhenLeaseQueueIsEmpty(t *testing.T) {
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

	itemType := openapiv1.ListQueueItemsParamsTypeRunning
	resp, err := a.ListQueueItems(ctx, openapiv1.ListQueueItemsRequestObject{
		Name: "fallback-q",
		Params: openapiv1.ListQueueItemsParams{
			Type: &itemType,
		},
	})
	require.NoError(t, err)

	listResp, ok := resp.(openapiv1.ListQueueItems200JSONResponse)
	require.True(t, ok)
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, "fresh-run", listResp.Items[0].DagRunId)
}

func TestDeleteQueueItemsClearsQueuedRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	queueStore := filequeue.New(filepath.Join(tmpDir, "queues"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	dag := &core.DAG{
		Name:  "clear-queue-dag",
		Queue: "clear-q",
		Steps: []core.Step{{Name: "step", Command: "echo hi"}},
	}
	runRef := createQueuedQueueRun(t, ctx, dagRunStore, queueStore, dag, "queued-run", core.Queued)

	a := &API{
		dagRunStore: dagRunStore,
		queueStore:  queueStore,
		procStore:   procStore,
		config: &config.Config{
			Server: config.Server{
				Permissions: map[config.Permission]bool{
					config.PermissionRunDAGs: true,
				},
			},
		},
	}

	resp, err := a.DeleteQueueItems(ctx, openapiv1.DeleteQueueItemsRequestObject{Name: dag.ProcGroup()})
	require.NoError(t, err)
	_, ok := resp.(openapiv1.DeleteQueueItems204Response)
	require.True(t, ok)

	count, err := queueStore.Len(ctx, dag.ProcGroup())
	require.NoError(t, err)
	assert.Zero(t, count)

	_, err = dagRunStore.FindAttempt(ctx, runRef)
	require.Error(t, err)
	assert.True(t, errors.Is(err, exec.ErrDAGRunIDNotFound) || errors.Is(err, exec.ErrNoStatusData))
}

func TestDeleteQueueItemsDropsStaleQueueItemWithoutChangingRunningStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	queueStore := filequeue.New(filepath.Join(tmpDir, "queues"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))

	dag := &core.DAG{
		Name:  "running-queue-dag",
		Queue: "running-q",
		Steps: []core.Step{{Name: "step", Command: "echo hi"}},
	}
	runRef := createQueuedQueueRun(t, ctx, dagRunStore, queueStore, dag, "running-run", core.Running)

	a := &API{
		dagRunStore: dagRunStore,
		queueStore:  queueStore,
		procStore:   procStore,
		config: &config.Config{
			Server: config.Server{
				Permissions: map[config.Permission]bool{
					config.PermissionRunDAGs: true,
				},
			},
		},
	}

	resp, err := a.DeleteQueueItems(ctx, openapiv1.DeleteQueueItemsRequestObject{Name: dag.ProcGroup()})
	require.NoError(t, err)
	_, ok := resp.(openapiv1.DeleteQueueItems204Response)
	require.True(t, ok)

	count, err := queueStore.Len(ctx, dag.ProcGroup())
	require.NoError(t, err)
	assert.Zero(t, count)

	attempt, err := dagRunStore.FindAttempt(ctx, runRef)
	require.NoError(t, err)
	status, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, core.Running, status.Status)
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
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	dag *core.DAG,
	dagRunID string,
	status core.Status,
) exec.DAGRunRef {
	t.Helper()

	runRef := exec.NewDAGRunRef(dag.Name, dagRunID)
	attempt, err := dagRunStore.CreateAttempt(ctx, dag, time.Now().UTC(), dagRunID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, attempt.Open(ctx))
	defer func() {
		require.NoError(t, attempt.Close(ctx))
	}()

	runStatus := exec.InitialStatus(dag)
	runStatus.Status = status
	runStatus.DAGRunID = dagRunID
	runStatus.AttemptID = attempt.ID()
	runStatus.CreatedAt = time.Now().UnixMilli()
	logPath := filepath.Join(t.TempDir(), dagRunID+".log")
	require.NoError(t, os.WriteFile(logPath, []byte(""), 0o600))
	runStatus.Log = logPath
	if status != core.Queued {
		runStatus.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}

	require.NoError(t, attempt.Write(ctx, runStatus))
	require.NoError(t, queueStore.Enqueue(ctx, dag.ProcGroup(), exec.QueuePriorityLow, runRef))

	return runRef
}
