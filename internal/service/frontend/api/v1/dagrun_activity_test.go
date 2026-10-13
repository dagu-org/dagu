// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	openapiv1 "github.com/dagucloud/dagu/v2/api/v1"
	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	runtimepkg "github.com/dagucloud/dagu/v2/internal/runtime"
	"github.com/dagucloud/dagu/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type activityFixture struct {
	t          *testing.T
	api        *API
	repository *persis.DAGRunRepository
}

func newActivityFixture(t *testing.T) *activityFixture {
	t.Helper()
	repository := testutil.NewFileDAGRunRepository(t.TempDir(), persis.DAGRunRepositoryOptions{})
	cfg := &config.Config{}
	return &activityFixture{
		t:          t,
		repository: repository,
		api: &API{
			dagRunRepository: repository,
			dagRunMgr:        runtimepkg.NewManager(repository, nil, cfg),
			config:           cfg,
		},
	}
}

// addRun saves a run that started at start and ran for duration.
func (f *activityFixture) addRun(name, id string, status ir.Status, start time.Time, duration time.Duration, nodes ...*ir.Node) {
	f.t.Helper()
	ctx := f.t.Context()
	dag := &ir.DAG{Name: name}
	attempt, err := f.repository.CreateAttempt(ctx, dag, start, id, persis.DAGRunCreateAttemptOptions{})
	require.NoError(f.t, err)
	require.NoError(f.t, attempt.Open(ctx))
	runStatus := ir.InitialStatus(dag)
	runStatus.DAGRunID = id
	runStatus.AttemptID = attempt.ID()
	runStatus.Status = status
	runStatus.StartedAt = start.Format(time.RFC3339)
	if duration > 0 {
		runStatus.FinishedAt = start.Add(duration).Format(time.RFC3339)
	}
	runStatus.Nodes = nodes
	require.NoError(f.t, attempt.Write(ctx, runStatus))
	require.NoError(f.t, attempt.Close(ctx))
}

func (f *activityFixture) activity(from, to time.Time) openapiv1.DAGRunActivity {
	f.t.Helper()
	fromUnix, toUnix := from.Unix(), to.Unix()
	resp, err := f.api.GetDAGRunActivity(f.t.Context(), openapiv1.GetDAGRunActivityRequestObject{
		Params: openapiv1.GetDAGRunActivityParams{FromDate: &fromUnix, ToDate: &toUnix},
	})
	require.NoError(f.t, err)
	activity, ok := resp.(openapiv1.GetDAGRunActivity200JSONResponse)
	require.True(f.t, ok)
	return openapiv1.DAGRunActivity(activity)
}

func activityCount(activity openapiv1.DAGRunActivity, status ir.Status) int {
	for _, count := range activity.Counts {
		if count.Status == openapiv1.Status(status) {
			return count.Count
		}
	}
	return 0
}

func TestDAGRunActivitySummarizesWindow(t *testing.T) {
	f := newActivityFixture(t)
	day := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	f.addRun("etl", "etl-1", ir.Succeeded, day.Add(1*time.Hour), 10*time.Second)
	f.addRun("etl", "etl-2", ir.Succeeded, day.Add(2*time.Hour), 20*time.Second)
	f.addRun("etl", "etl-3", ir.Succeeded, day.Add(3*time.Hour), 30*time.Second)
	f.addRun("etl", "etl-4", ir.Failed, day.Add(4*time.Hour), 5*time.Second,
		&ir.Node{Step: ir.Step{Name: "extract"}, Status: ir.NodeSucceeded},
		&ir.Node{Step: ir.Step{Name: "load"}, Status: ir.NodeFailed, Error: "\x1b[31mconnection refused\x1b[0m\nretry later"},
	)
	f.addRun("ping", "ping-1", ir.Aborted, day.Add(5*time.Hour), time.Second)
	f.addRun("ping", "ping-2", ir.Succeeded, day.Add(6*time.Hour), time.Second)
	f.addRun("etl", "outside", ir.Failed, day.Add(-time.Hour), time.Second)

	activity := f.activity(day, day.Add(24*time.Hour))

	assert.Equal(t, 4, activityCount(activity, ir.Succeeded))
	assert.Equal(t, 1, activityCount(activity, ir.Failed))
	assert.Equal(t, 1, activityCount(activity, ir.Aborted))
	assert.Equal(t, 900, activity.BucketSeconds)
	assert.False(t, activity.Truncated)
	require.Len(t, activity.Workflows, 2)

	etl := activity.Workflows[0]
	assert.Equal(t, "etl", etl.Name)
	assert.Equal(t, 4, etl.RunCount)
	assert.Len(t, etl.Runs, 4)
	assert.Equal(t, "etl-4", etl.Latest.DagRunId)
	assert.Equal(t, "load", valueOf(etl.Latest.FailedStep))
	assert.Equal(t, "connection refused", valueOf(etl.Latest.Error))
	assert.Equal(t, 20, valueOf(etl.MedianSuccessSeconds))
	assert.Equal(t, 1, etl.FailedCount)
	// 15-minute buckets: runs at 01:00, 02:00, 03:00, and a failure at 04:00.
	assert.Equal(t, "....s...s...s...f", etl.Strip[:17])
	assert.Len(t, etl.Strip, activityBucketCount)

	ping := activity.Workflows[1]
	assert.Equal(t, "ping", ping.Name)
	assert.Equal(t, "ping-2", ping.Latest.DagRunId)
	assert.Nil(t, ping.MedianSuccessSeconds)
}

// A busy DAG lists only its unsuccessful runs and its latest run, and those
// runs come before routine ones when the list is capped.
func TestSelectActivityRunsTrimsBusyDAGs(t *testing.T) {
	start := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	busy := &activityWorkflow{name: "ping"}
	for i := range activityFewRuns + 6 {
		status := ir.Succeeded
		if i == 10 {
			status = ir.Aborted
		}
		busy.runs = append(busy.runs, activityRun{
			id:     fmt.Sprintf("ping-%02d", i),
			status: status,
			at:     start.Add(time.Duration(i) * time.Minute),
		})
	}
	quiet := &activityWorkflow{name: "etl", runs: []activityRun{
		{id: "etl-1", status: ir.Succeeded, at: start.Add(2 * time.Hour)},
		{id: "etl-2", status: ir.Succeeded, at: start.Add(3 * time.Hour)},
	}}

	listed := selectActivityRuns(map[string]*activityWorkflow{"ping": busy, "etl": quiet})

	var ids []string
	for _, entry := range listed {
		ids = append(ids, entry.run.id)
	}
	assert.Equal(t, []string{"ping-10", "etl-2", "etl-1", "ping-29"}, ids)
}

func TestDAGRunActivityListsActiveRuns(t *testing.T) {
	f := newActivityFixture(t)
	day := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	f.addRun("train", "train-1", ir.Running, day.Add(2*time.Hour), 0)
	f.addRun("approve", "approve-1", ir.Waiting, day.Add(time.Hour), 0)
	f.addRun("report", "report-1", ir.Succeeded, day.Add(3*time.Hour), time.Minute)

	activity := f.activity(day, day.Add(24*time.Hour))

	require.Len(t, activity.Active, 2)
	assert.Equal(t, "approve-1", activity.Active[0].DagRunId)
	assert.Equal(t, "approve", valueOf(activity.Active[0].Name))
	assert.Equal(t, "train-1", activity.Active[1].DagRunId)
}

// aliveProcs reports the given runs as having live processes.
type aliveProcs struct {
	processRepository
	refs []ir.DAGRunRef
}

func (p aliveProcs) ListAllAlive(context.Context) (map[string][]ir.DAGRunRef, error) {
	return map[string][]ir.DAGRunRef{"default": p.refs}, nil
}

// Runs that started before the window appear while their process lives, up
// to a bound on the status files read for them.
func TestDAGRunActivityListsLiveRunsFromBeforeWindow(t *testing.T) {
	f := newActivityFixture(t)
	day := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	var refs []ir.DAGRunRef
	for i := range activityLiveRunReads + 1 {
		id := fmt.Sprintf("train-%02d", i)
		f.addRun("train", id, ir.Running, day.Add(-time.Hour).Add(time.Duration(i)*time.Second), 0)
		refs = append(refs, ir.NewDAGRunRef("train", id))
	}
	f.api.procRepository = aliveProcs{refs: refs}

	activity := f.activity(day, day.Add(24*time.Hour))

	require.Len(t, activity.Active, activityLiveRunReads)
	assert.Equal(t, "train-00", activity.Active[0].DagRunId)
	assert.Equal(t, "train", valueOf(activity.Active[0].Name))
	assert.Empty(t, activity.Workflows)
}

func TestDAGRunActivityRejectsInvalidWindows(t *testing.T) {
	f := newActivityFixture(t)
	now := time.Now()
	tests := []struct {
		name   string
		params openapiv1.GetDAGRunActivityParams
	}{
		{name: "EndsBeforeStart", params: openapiv1.GetDAGRunActivityParams{
			FromDate: new(now.Unix()), ToDate: new(now.Add(-time.Hour).Unix()),
		}},
		{name: "LongerThanAWeek", params: openapiv1.GetDAGRunActivityParams{
			FromDate: new(now.Add(-8 * 24 * time.Hour).Unix()), ToDate: new(now.Unix()),
		}},
		{name: "TooManyHours", params: openapiv1.GetDAGRunActivityParams{Hours: new(49)}},
		{name: "NoHours", params: openapiv1.GetDAGRunActivityParams{Hours: new(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.api.GetDAGRunActivity(t.Context(), openapiv1.GetDAGRunActivityRequestObject{Params: tt.params})

			var apiErr *Error
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
		})
	}
}

// A live window ends at the next bucket boundary, so it stays the same, and
// so does its payload, until that boundary passes.
func TestDAGRunActivityAlignsLiveWindowToBuckets(t *testing.T) {
	f := newActivityFixture(t)
	now := time.Date(2026, 3, 2, 10, 7, 30, 0, time.UTC)

	query, err := f.api.newActivityQuery(t.Context(), activityParams{}, now)

	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, query.bucket)
	assert.Equal(t, time.Date(2026, 3, 2, 10, 15, 0, 0, time.UTC), query.to.UTC())
	assert.Equal(t, query.to.Add(-24*time.Hour), query.from)
}

func TestDAGRunActivityStreamTakesOnlyLiveWindows(t *testing.T) {
	f := newActivityFixture(t)

	require.NoError(t, f.api.ValidateDAGRunActivityQuery(t.Context(), "hours=6"))

	err := f.api.ValidateDAGRunActivityQuery(t.Context(), "fromDate=1")
	var apiErr *Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
}
