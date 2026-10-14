// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dagrun_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/dagucloud/dagu/v2/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestLatestStatusesReturnsNewestRunPerDAG(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repository := newFileRepository(baseDir, persis.DAGRunRepositoryOptions{})
	cutoff := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	writeLatestStatusRun(t, repository, "recovered", "failed", cutoff.Add(time.Minute), ir.Failed)
	writeLatestStatusRun(t, repository, "recovered", "fixed", cutoff.Add(2*time.Minute), ir.Succeeded)
	writeLatestStatusRun(t, repository, "broken", "boom", cutoff.Add(3*time.Minute), ir.Failed, "team=ops", "workspace=ops")
	writeLatestStatusRun(t, repository, "quiet", "old", cutoff.Add(-time.Minute), ir.Failed)

	latest := func(query persis.DAGRunLatestStatusesQuery) []string {
		t.Helper()
		query.NotBefore = persis.NewUTC(cutoff)
		statuses, err := repository.LatestStatuses(ctx, query)
		require.NoError(t, err)
		runs := make([]string, 0, len(statuses))
		for _, status := range statuses {
			runs = append(runs, status.Name+"/"+status.DAGRunID+"/"+status.Status.String())
		}
		slices.Sort(runs)
		return runs
	}

	require.Equal(t, []string{"broken/boom/failed", "recovered/fixed/succeeded"}, latest(persis.DAGRunLatestStatusesQuery{}))
	require.Equal(t, []string{"broken/boom/failed"}, latest(persis.DAGRunLatestStatusesQuery{Labels: []string{"team=ops"}}))
	require.Equal(t, []string{"broken/boom/failed"}, latest(persis.DAGRunLatestStatusesQuery{
		WorkspaceFilter: &workspace.WorkspaceFilter{Enabled: true, Workspaces: []string{"ops"}},
	}))

	// A lost pointer is rebuilt from the run directories.
	require.NoError(t, os.Remove(filepath.Join(baseDir, "recovered", "dag-runs", ".dagrun.latest")))
	require.Equal(t, []string{"broken/boom/failed", "recovered/fixed/succeeded"}, latest(persis.DAGRunLatestStatusesQuery{}))
}

func writeLatestStatusRun(
	t *testing.T,
	repository *persis.DAGRunRepository,
	name, runID string,
	createdAt time.Time,
	status ir.Status,
	labels ...string,
) {
	t.Helper()

	ctx := context.Background()
	attempt, err := repository.CreateAttempt(ctx, &ir.DAG{Name: name}, createdAt, runID, persis.DAGRunCreateAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, attempt.Open(ctx))
	require.NoError(t, attempt.Write(ctx, ir.DAGRunStatus{
		Name:      name,
		DAGRunID:  runID,
		AttemptID: attempt.ID(),
		Status:    status,
		Labels:    labels,
		StartedAt: createdAt.Format(time.RFC3339),
	}))
	require.NoError(t, attempt.Close(ctx))
}
