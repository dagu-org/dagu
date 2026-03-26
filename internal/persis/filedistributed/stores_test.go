// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatchTaskStore_ClaimRecycleAndSelectorFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDispatchTaskStore(filepath.Join(t.TempDir(), "distributed"))

	require.NoError(t, store.Enqueue(ctx, &coordinatorv1.Task{
		DagRunId:       "run-a",
		Target:         "dag-a",
		AttemptId:      "attempt-a",
		AttemptKey:     "attempt-key-a",
		WorkerSelector: map[string]string{"type": "gpu"},
	}))
	require.NoError(t, store.Enqueue(ctx, &coordinatorv1.Task{
		DagRunId:       "run-b",
		Target:         "dag-b",
		AttemptId:      "attempt-b",
		AttemptKey:     "attempt-key-b",
		WorkerSelector: map[string]string{"type": "cpu"},
	}))

	claimed, err := store.ClaimNext(ctx, exec.DispatchTaskClaim{
		WorkerID:     "worker-1",
		PollerID:     "poller-1",
		Labels:       map[string]string{"type": "cpu"},
		Owner:        exec.CoordinatorEndpoint{ID: "coord-a"},
		ClaimTimeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, "run-b", claimed.Task.DagRunId)
	assert.Equal(t, "coord-a", claimed.Task.OwnerCoordinatorId)
	assert.NotEmpty(t, claimed.Task.ClaimToken)

	// Claim should not be visible to a second poller until it expires.
	secondClaim, err := store.ClaimNext(ctx, exec.DispatchTaskClaim{
		WorkerID:     "worker-2",
		PollerID:     "poller-2",
		Labels:       map[string]string{"type": "cpu"},
		Owner:        exec.CoordinatorEndpoint{ID: "coord-b"},
		ClaimTimeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	assert.Nil(t, secondClaim)

	var reclaimed *exec.ClaimedDispatchTask
	require.Eventually(t, func() bool {
		var claimErr error
		reclaimed, claimErr = store.ClaimNext(ctx, exec.DispatchTaskClaim{
			WorkerID:     "worker-2",
			PollerID:     "poller-2",
			Labels:       map[string]string{"type": "cpu"},
			Owner:        exec.CoordinatorEndpoint{ID: "coord-b"},
			ClaimTimeout: 50 * time.Millisecond,
		})
		require.NoError(t, claimErr)
		return reclaimed != nil
	}, time.Second, 10*time.Millisecond)
	require.NotNil(t, reclaimed)
	assert.Equal(t, "run-b", reclaimed.Task.DagRunId)
	assert.Equal(t, "coord-b", reclaimed.Task.OwnerCoordinatorId)

	_, err = store.GetClaim(ctx, claimed.ClaimToken)
	assert.ErrorIs(t, err, exec.ErrDispatchTaskNotFound)

	// GPU task remains claimable only by matching workers.
	gpuClaim, err := store.ClaimNext(ctx, exec.DispatchTaskClaim{
		WorkerID:     "worker-3",
		PollerID:     "poller-3",
		Labels:       map[string]string{"type": "gpu"},
		Owner:        exec.CoordinatorEndpoint{ID: "coord-c"},
		ClaimTimeout: time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, gpuClaim)
	assert.Equal(t, "run-a", gpuClaim.Task.DagRunId)
}

func TestDispatchTaskStore_ConcurrentClaimIsExclusive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDispatchTaskStore(filepath.Join(t.TempDir(), "distributed"))

	require.NoError(t, store.Enqueue(ctx, &coordinatorv1.Task{
		DagRunId:       "run-exclusive",
		Target:         "dag-exclusive",
		AttemptId:      "attempt-exclusive",
		AttemptKey:     "attempt-key-exclusive",
		WorkerSelector: map[string]string{"type": "cpu"},
	}))

	const pollers = 16
	results := make(chan *exec.ClaimedDispatchTask, pollers)
	errs := make(chan error, pollers)

	var wg sync.WaitGroup
	for i := range pollers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			claimed, err := store.ClaimNext(ctx, exec.DispatchTaskClaim{
				WorkerID:     "worker-1",
				PollerID:     "poller-" + string(rune('a'+idx)),
				Labels:       map[string]string{"type": "cpu"},
				Owner:        exec.CoordinatorEndpoint{ID: "coord-a"},
				ClaimTimeout: time.Second,
			})
			errs <- err
			results <- claimed
		}(i)
	}
	wg.Wait()
	close(errs)
	close(results)

	for err := range errs {
		require.NoError(t, err)
	}

	claimedCount := 0
	for claimed := range results {
		if claimed != nil {
			claimedCount++
		}
	}

	assert.Equal(t, 1, claimedCount)
}

func TestWorkerHeartbeatStore_UpsertListAndDeleteStale(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewWorkerHeartbeatStore(filepath.Join(t.TempDir(), "distributed"))

	staleAt := time.Now().Add(-2 * time.Minute).UTC()
	require.NoError(t, store.Upsert(ctx, exec.WorkerHeartbeatRecord{
		WorkerID:        "worker-stale",
		Labels:          map[string]string{"type": "cpu"},
		LastHeartbeatAt: staleAt.UnixMilli(),
	}))
	require.NoError(t, store.Upsert(ctx, exec.WorkerHeartbeatRecord{
		WorkerID:        "worker-fresh",
		Labels:          map[string]string{"type": "gpu"},
		LastHeartbeatAt: time.Now().UTC().UnixMilli(),
	}))

	records, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, records, 2)

	removed, err := store.DeleteStale(ctx, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	records, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "worker-fresh", records[0].WorkerID)
}

func TestDAGRunLeaseStore_UpsertTouchListAndDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDAGRunLeaseStore(filepath.Join(t.TempDir(), "distributed"))

	claimedAt := time.Now().Add(-time.Minute).UTC()
	require.NoError(t, store.Upsert(ctx, exec.DAGRunLease{
		AttemptKey:      "attempt-key-1",
		DAGRun:          exec.NewDAGRunRef("dag-a", "run-1"),
		Root:            exec.NewDAGRunRef("dag-a", "run-1"),
		AttemptID:       "attempt-1",
		QueueName:       "queue-a",
		WorkerID:        "worker-1",
		ClaimedAt:       claimedAt.UnixMilli(),
		LastHeartbeatAt: claimedAt.UnixMilli(),
	}))
	require.NoError(t, store.Upsert(ctx, exec.DAGRunLease{
		AttemptKey:      "attempt-key-2",
		DAGRun:          exec.NewDAGRunRef("dag-b", "run-2"),
		Root:            exec.NewDAGRunRef("dag-b", "run-2"),
		AttemptID:       "attempt-2",
		QueueName:       "queue-b",
		WorkerID:        "worker-2",
		LastHeartbeatAt: time.Now().UTC().UnixMilli(),
	}))

	leases, err := store.ListByQueue(ctx, "queue-a")
	require.NoError(t, err)
	require.Len(t, leases, 1)
	assert.Equal(t, "attempt-key-1", leases[0].AttemptKey)

	touchedAt := time.Now().UTC()
	require.NoError(t, store.Touch(ctx, "attempt-key-1", touchedAt))

	lease, err := store.Get(ctx, "attempt-key-1")
	require.NoError(t, err)
	assert.Equal(t, claimedAt.UnixMilli(), lease.ClaimedAt)
	assert.GreaterOrEqual(t, lease.LastHeartbeatAt, touchedAt.UnixMilli())

	require.NoError(t, store.Delete(ctx, "attempt-key-1"))
	_, err = store.Get(ctx, "attempt-key-1")
	assert.ErrorIs(t, err, exec.ErrDAGRunLeaseNotFound)
}

func TestDAGRunLeaseStore_ConcurrentTouchPreservesLatestHeartbeat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDAGRunLeaseStore(filepath.Join(t.TempDir(), "distributed"))

	require.NoError(t, store.Upsert(ctx, exec.DAGRunLease{
		AttemptKey:      "attempt-key-concurrent",
		DAGRun:          exec.NewDAGRunRef("dag-a", "run-1"),
		Root:            exec.NewDAGRunRef("dag-a", "run-1"),
		AttemptID:       "attempt-1",
		QueueName:       "queue-a",
		WorkerID:        "worker-1",
		LastHeartbeatAt: time.Now().Add(-time.Minute).UTC().UnixMilli(),
	}))

	latest := time.Now().Add(time.Second).UTC()
	var wg sync.WaitGroup
	errCh := make(chan error, 3)
	for range 3 {
		wg.Go(func() {
			errCh <- store.Touch(ctx, "attempt-key-concurrent", latest)
		})
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	lease, err := store.Get(ctx, "attempt-key-concurrent")
	require.NoError(t, err)
	require.NotNil(t, lease)
	assert.Equal(t, latest.UnixMilli(), lease.LastHeartbeatAt)
}
