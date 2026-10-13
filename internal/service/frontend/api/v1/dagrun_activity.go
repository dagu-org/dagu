// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/api/v1"
	"github.com/dagucloud/dagu/v2/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/pagination"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/dagucloud/dagu/v2/internal/workspace"
)

const (
	// activityBucketCount is how many equal buckets split an activity window.
	activityBucketCount  = 96
	activityDefaultHours = 24
	// activityMaxLiveHours bounds a window that ends now, which live
	// updates recompute on every run status change.
	activityMaxLiveHours = 48
	// activityMaxRange bounds a fixed window read on request.
	activityMaxRange = 7 * 24 * time.Hour
	// activityFewRuns is the run count up to which a DAG lists every run.
	activityFewRuns = 24
	// activityRunCap bounds the runs listed across all DAGs.
	activityRunCap = 300
	// activityFailureDetails bounds the status files read for failure details.
	activityFailureDetails = 20
	activityUpcomingLimit  = 5
	activityErrorMaxBytes  = 200
)

// GetDAGRunActivity summarizes the DAG-runs of a time window.
func (a *API) GetDAGRunActivity(ctx context.Context, request api.GetDAGRunActivityRequestObject) (api.GetDAGRunActivityResponseObject, error) {
	params := request.Params
	query, err := a.newActivityQuery(ctx, activityParams{
		hours:     params.Hours,
		fromDate:  params.FromDate,
		toDate:    params.ToDate,
		labels:    params.Labels,
		workspace: params.Workspace,
	}, time.Now())
	if err != nil {
		return nil, err
	}
	activity, err := a.dagRunActivity(ctx, query)
	if err != nil {
		return nil, err
	}
	return api.GetDAGRunActivity200JSONResponse(activity), nil
}

// GetDAGRunActivityData returns the activity of a window that ends now for
// SSE. The identifier is the REST query string without fromDate and toDate.
func (a *API) GetDAGRunActivityData(ctx context.Context, queryString string) (any, error) {
	query, err := a.activityQueryFromString(ctx, queryString)
	if err != nil {
		return nil, err
	}
	return a.dagRunActivity(ctx, query)
}

// ValidateDAGRunActivityQuery returns a 400 *Error when GetDAGRunActivityData
// would reject the query string.
func (a *API) ValidateDAGRunActivityQuery(ctx context.Context, queryString string) error {
	_, err := a.activityQueryFromString(ctx, queryString)
	return err
}

func (a *API) activityQueryFromString(ctx context.Context, queryString string) (activityQuery, error) {
	values, err := url.ParseQuery(queryString)
	if err != nil {
		return activityQuery{}, activityBadRequest(fmt.Sprintf("invalid query parameters: %s", err))
	}
	if values.Has("fromDate") || values.Has("toDate") {
		return activityQuery{}, activityBadRequest("live activity takes hours, not fromDate or toDate")
	}
	var params activityParams
	if raw := values.Get("hours"); raw != "" {
		hours, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return activityQuery{}, activityBadRequest(fmt.Sprintf("invalid hours: %q", raw))
		}
		params.hours = &hours
	}
	if raw := values.Get("labels"); raw != "" {
		params.labels = &raw
	}
	params.workspace = workspaceParamFromValues(values)
	return a.newActivityQuery(ctx, params, time.Now())
}

type activityParams struct {
	hours     *int
	fromDate  *int64
	toDate    *int64
	labels    *string
	workspace *api.Workspace
}

// activityQuery is a validated activity window and its filters.
type activityQuery struct {
	from            time.Time
	to              time.Time
	bucket          time.Duration
	labels          []string
	workspaceFilter *workspace.WorkspaceFilter
}

func (a *API) newActivityQuery(ctx context.Context, params activityParams, now time.Time) (activityQuery, error) {
	var query activityQuery
	if params.fromDate != nil {
		query.from = time.Unix(*params.fromDate, 0)
		query.to = now
		if params.toDate != nil {
			query.to = time.Unix(*params.toDate, 0)
		}
		span := query.to.Sub(query.from)
		if span <= 0 || span > activityMaxRange {
			return activityQuery{}, activityBadRequest("the window must end after it starts and span at most 7 days")
		}
		query.bucket = max(time.Second, (span+activityBucketCount*time.Second-1)/activityBucketCount).Truncate(time.Second)
	} else {
		hours := activityDefaultHours
		if params.hours != nil {
			hours = *params.hours
		}
		if hours < 1 || hours > activityMaxLiveHours {
			return activityQuery{}, activityBadRequest(fmt.Sprintf("hours must be between 1 and %d", activityMaxLiveHours))
		}
		// Aligning to bucket boundaries keeps the window, and so the payload,
		// unchanged until the next boundary.
		query.bucket = time.Duration(hours) * time.Hour / activityBucketCount
		query.to = now.Truncate(query.bucket).Add(query.bucket)
		query.from = query.to.Add(-activityBucketCount * query.bucket)
	}
	query.labels = parseCommaSeparatedLabels(params.labels)
	filter, err := a.workspaceFilterForParams(ctx, params.workspace)
	if err != nil {
		return activityQuery{}, err
	}
	query.workspaceFilter = filter
	return query, nil
}

func activityBadRequest(message string) *Error {
	return &Error{
		HTTPStatus: http.StatusBadRequest,
		Code:       api.ErrorCodeBadRequest,
		Message:    message,
	}
}

// activityRun is one run seen in the window.
type activityRun struct {
	id       string
	status   ir.Status
	at       time.Time
	started  int64
	finished int64
}

// activityWorkflow accumulates the runs of one DAG.
type activityWorkflow struct {
	name             string
	runs             []activityRun
	failedCount      int
	strip            []byte
	successDurations []int64
}

// Strip characters name the most severe run in a bucket.
const (
	stripEmpty     = '.'
	stripSucceeded = 's'
	stripActive    = 'r'
	stripPartial   = 'p'
	stripAborted   = 'a'
	stripFailed    = 'f'
)

// stripSeverity orders strip characters; a bucket shows its most severe run.
var stripSeverity = map[byte]int{
	stripEmpty:     0,
	stripSucceeded: 1,
	stripActive:    2,
	stripPartial:   3,
	stripAborted:   4,
	stripFailed:    5,
}

func (a *API) dagRunActivity(ctx context.Context, query activityQuery) (api.DAGRunActivity, error) {
	return withDAGRunReadTimeout(ctx, dagRunReadRequestInfo{
		endpoint: "/dag-run-activity",
	}, func(readCtx context.Context) (api.DAGRunActivity, error) {
		return a.readDAGRunActivity(readCtx, query)
	})
}

func (a *API) readDAGRunActivity(ctx context.Context, query activityQuery) (api.DAGRunActivity, error) {
	counts := make(map[ir.Status]int)
	workflows := make(map[string]*activityWorkflow)
	err := a.dagRunRepository.VisitStatuses(ctx, persis.DAGRunListOptions{
		From:            persis.NewUTC(query.from),
		To:              persis.NewUTC(query.to),
		Labels:          query.labels,
		WorkspaceFilter: query.workspaceFilter,
	}, func(status *ir.DAGRunStatus) error {
		counts[status.Status]++
		workflow := workflows[status.Name]
		if workflow == nil {
			workflow = &activityWorkflow{
				name:  status.Name,
				strip: []byte(strings.Repeat(string(stripEmpty), activityBucketCount)),
			}
			workflows[status.Name] = workflow
		}
		run := newActivityRun(status)
		workflow.runs = append(workflow.runs, run)
		workflow.addToStrip(query, run)
		if status.Status == ir.Failed || status.Status == ir.Rejected {
			workflow.failedCount++
		}
		if status.Status == ir.Succeeded && run.started > 0 && run.finished >= run.started {
			workflow.successDurations = append(workflow.successDurations, run.finished-run.started)
		}
		return nil
	})
	if err != nil {
		return api.DAGRunActivity{}, fmt.Errorf("error reading dag-run activity: %w", err)
	}

	result := api.DAGRunActivity{
		From:          query.from.Unix(),
		To:            query.to.Unix(),
		BucketSeconds: int(query.bucket / time.Second),
		Counts:        activityCounts(counts),
		Workflows:     make([]api.DAGRunActivityWorkflow, 0, len(workflows)),
	}

	names := make([]string, 0, len(workflows))
	for name := range workflows {
		names = append(names, name)
	}
	slices.Sort(names)
	listed := selectActivityRuns(workflows)
	result.Truncated = len(listed) > activityRunCap
	if result.Truncated {
		listed = listed[:activityRunCap]
	}
	runsByWorkflow := make(map[string][]activityRun, len(workflows))
	for _, entry := range listed {
		runsByWorkflow[entry.workflow] = append(runsByWorkflow[entry.workflow], entry.run)
	}

	for _, name := range names {
		workflow := workflows[name]
		runs := runsByWorkflow[name]
		slices.SortFunc(runs, compareActivityRunsNewestFirst)
		item := api.DAGRunActivityWorkflow{
			Name:        name,
			RunCount:    len(workflow.runs),
			FailedCount: workflow.failedCount,
			Strip:       string(workflow.strip),
			Runs:        make([]api.DAGRunActivityRun, 0, len(runs)),
			Latest:      toActivityRun(workflow.runs[0], ""),
		}
		for _, run := range runs {
			item.Runs = append(item.Runs, toActivityRun(run, ""))
		}
		if median, ok := medianSeconds(workflow.successDurations); ok {
			item.MedianSuccessSeconds = &median
		}
		result.Workflows = append(result.Workflows, item)
	}
	a.addFailureDetails(ctx, result.Workflows)
	result.Active = a.activeRuns(ctx, query, workflows)
	result.Upcoming = a.upcomingRuns(ctx, query)
	return result, nil
}

func newActivityRun(status *ir.DAGRunStatus) activityRun {
	run := activityRun{id: status.DAGRunID, status: status.Status}
	if started, err := stringutil.ParseTime(status.StartedAt); err == nil && !started.IsZero() {
		run.started = started.Unix()
		run.at = started
	}
	if finished, err := stringutil.ParseTime(status.FinishedAt); err == nil && !finished.IsZero() {
		run.finished = finished.Unix()
	}
	if run.at.IsZero() {
		if queued, err := stringutil.ParseTime(status.QueuedAt); err == nil && !queued.IsZero() {
			run.at = queued
		} else if status.CreatedAt > 0 {
			run.at = time.UnixMilli(status.CreatedAt)
		}
	}
	return run
}

func (w *activityWorkflow) addToStrip(query activityQuery, run activityRun) {
	index := 0
	if !run.at.IsZero() {
		index = int(run.at.Sub(query.from) / query.bucket)
	}
	index = min(max(index, 0), activityBucketCount-1)
	if mark := stripMark(run.status); stripSeverity[mark] > stripSeverity[w.strip[index]] {
		w.strip[index] = mark
	}
}

func stripMark(status ir.Status) byte {
	switch status {
	case ir.Succeeded:
		return stripSucceeded
	case ir.PartiallySucceeded:
		return stripPartial
	case ir.Failed, ir.Rejected:
		return stripFailed
	case ir.Aborted:
		return stripAborted
	case ir.NotStarted, ir.Running, ir.Queued, ir.Waiting:
		return stripActive
	}
	return stripEmpty
}

func activityCounts(counts map[ir.Status]int) []api.DAGRunActivityCount {
	result := make([]api.DAGRunActivityCount, 0, len(counts))
	for status, count := range counts {
		result = append(result, api.DAGRunActivityCount{Status: api.Status(status), Count: count})
	}
	slices.SortFunc(result, func(left, right api.DAGRunActivityCount) int {
		return cmp.Compare(left.Status, right.Status)
	})
	return result
}

// isNotableRun reports whether a run is listed even for a busy DAG.
func isNotableRun(status ir.Status) bool {
	return status != ir.Succeeded
}

type listedActivityRun struct {
	workflow string
	run      activityRun
	notable  bool
}

// selectActivityRuns sorts each DAG's runs newest first and returns the runs
// to list, most important first: every run of a DAG with few runs, otherwise
// its notable runs and its latest run.
func selectActivityRuns(workflows map[string]*activityWorkflow) []listedActivityRun {
	var listed []listedActivityRun
	for name, workflow := range workflows {
		slices.SortFunc(workflow.runs, compareActivityRunsNewestFirst)
		for i, run := range workflow.runs {
			notable := isNotableRun(run.status)
			if len(workflow.runs) > activityFewRuns && !notable && i > 0 {
				continue
			}
			listed = append(listed, listedActivityRun{workflow: name, run: run, notable: notable})
		}
	}
	slices.SortFunc(listed, func(left, right listedActivityRun) int {
		if left.notable != right.notable {
			if left.notable {
				return -1
			}
			return 1
		}
		if c := compareActivityRunsNewestFirst(left.run, right.run); c != 0 {
			return c
		}
		return cmp.Compare(left.workflow, right.workflow)
	})
	return listed
}

func compareActivityRunsNewestFirst(left, right activityRun) int {
	if c := right.at.Compare(left.at); c != 0 {
		return c
	}
	return cmp.Compare(right.id, left.id)
}

func toActivityRun(run activityRun, name string) api.DAGRunActivityRun {
	result := api.DAGRunActivityRun{
		DagRunId: run.id,
		Status:   api.Status(run.status),
	}
	if name != "" {
		result.Name = &name
	}
	if run.started > 0 {
		result.StartedAt = &run.started
	} else if !run.at.IsZero() {
		at := run.at.Unix()
		result.StartedAt = &at
	}
	if run.finished > 0 {
		result.FinishedAt = &run.finished
	}
	return result
}

func medianSeconds(durations []int64) (int, bool) {
	if len(durations) < 3 {
		return 0, false
	}
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	return int(sorted[len(sorted)/2]), true
}

func isUnsuccessfulStatus(status ir.Status) bool {
	switch status {
	case ir.Failed, ir.Aborted, ir.Rejected, ir.PartiallySucceeded:
		return true
	case ir.NotStarted, ir.Running, ir.Succeeded, ir.Queued, ir.Waiting:
		return false
	}
	return false
}

// addFailureDetails names the failed step and its error on the latest run of
// the most recently failing DAGs. Index entries carry no step data, so each
// detail costs one cached status read.
func (a *API) addFailureDetails(ctx context.Context, workflows []api.DAGRunActivityWorkflow) {
	failing := make([]int, 0)
	for i, workflow := range workflows {
		if isUnsuccessfulStatus(ir.Status(workflow.Latest.Status)) {
			failing = append(failing, i)
		}
	}
	slices.SortFunc(failing, func(left, right int) int {
		return cmp.Compare(valueOf(workflows[right].Latest.StartedAt), valueOf(workflows[left].Latest.StartedAt))
	})
	for _, i := range failing[:min(len(failing), activityFailureDetails)] {
		latest := &workflows[i].Latest
		status, err := a.dagRunMgr.GetSavedStatus(ctx, ir.NewDAGRunRef(workflows[i].Name, latest.DagRunId))
		if err != nil {
			continue
		}
		step, message := firstFailure(status)
		if step != "" {
			latest.FailedStep = &step
		}
		if message != "" {
			latest.Error = &message
		}
	}
}

// firstFailure returns the first failed step in run order and its error, or
// the run-level error when no step failed.
func firstFailure(status *ir.DAGRunStatus) (string, string) {
	for _, node := range status.NodesInRunOrder() {
		if node != nil && node.Status == ir.NodeFailed {
			return node.Step.Name, shortError(node.Error)
		}
	}
	return "", shortError(status.Error)
}

func shortError(message string) string {
	message = strings.TrimSpace(stringutil.StripANSI(message))
	if first, _, found := strings.Cut(message, "\n"); found {
		message = first
	}
	return stringutil.TruncUTF8Bytes(message, activityErrorMaxBytes)
}

// activeRuns lists runs that are still in progress: those in the window and
// live processes of runs created before it. Oldest first.
func (a *API) activeRuns(ctx context.Context, query activityQuery, workflows map[string]*activityWorkflow) []api.DAGRunActivityRun {
	active := make([]api.DAGRunActivityRun, 0)
	seen := make(map[ir.DAGRunRef]struct{})
	for name, workflow := range workflows {
		for _, run := range workflow.runs {
			if isActiveRunStatus(run.status) {
				active = append(active, toActivityRun(run, name))
				seen[ir.NewDAGRunRef(name, run.id)] = struct{}{}
			}
		}
	}

	if a.procRepository != nil {
		alive, err := a.procRepository.ListAllAlive(ctx)
		if err == nil {
			labelFilters := parseLabelFilters(query.labels)
			for _, refs := range alive {
				for _, ref := range refs {
					if _, ok := seen[ref]; ok {
						continue
					}
					seen[ref] = struct{}{}
					status, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
					if err != nil || !isActiveRunStatus(status.Status) {
						continue
					}
					labels := ir.NewLabels(status.Labels)
					if !query.workspaceFilter.MatchesLabels(labels) || !labels.MatchesFilters(labelFilters) {
						continue
					}
					active = append(active, toActivityRun(newActivityRun(status), status.Name))
				}
			}
		}
	}

	slices.SortFunc(active, func(left, right api.DAGRunActivityRun) int {
		if c := cmp.Compare(valueOf(left.StartedAt), valueOf(right.StartedAt)); c != 0 {
			return c
		}
		return cmp.Compare(left.DagRunId, right.DagRunId)
	})
	return active
}

func isActiveRunStatus(status ir.Status) bool {
	switch status {
	case ir.Running, ir.Queued, ir.Waiting:
		return true
	case ir.NotStarted, ir.Failed, ir.Aborted, ir.Succeeded, ir.PartiallySucceeded, ir.Rejected:
		return false
	}
	return false
}

func parseLabelFilters(labels []string) []ir.LabelFilter {
	filters := make([]ir.LabelFilter, 0, len(labels))
	for _, label := range labels {
		if trimmed := strings.TrimSpace(label); trimmed != "" {
			filters = append(filters, ir.ParseLabelFilter(trimmed))
		}
	}
	return filters
}

// upcomingRuns returns the next scheduled runs of unsuspended DAGs.
func (a *API) upcomingRuns(ctx context.Context, query activityQuery) []api.DAGRunActivityUpcoming {
	upcoming := make([]api.DAGRunActivityUpcoming, 0, activityUpcomingLimit)
	if a.dagRepository == nil {
		return upcoming
	}
	now := time.Now()
	projection := a.nextRunProjection(ctx)
	page := pagination.NewPaginator(1, activityUpcomingLimit)
	result, _, err := a.dagRepository.List(ctx, persis.DAGListOptions{
		Paginator:         &page,
		Labels:            query.labels,
		ActiveOnly:        true,
		Sort:              "nextRun",
		Order:             "asc",
		Time:              &now,
		NextRunProjection: projection,
		WorkspaceFilter:   query.workspaceFilter,
	})
	if err != nil {
		return upcoming
	}
	for _, item := range result.Items {
		nextRun := projection(item.DAG, now)
		if nextRun.IsZero() {
			continue
		}
		upcoming = append(upcoming, api.DAGRunActivityUpcoming{
			Name:     item.Name,
			FileName: item.ID,
			NextRun:  nextRun.Unix(),
		})
	}
	return upcoming
}
