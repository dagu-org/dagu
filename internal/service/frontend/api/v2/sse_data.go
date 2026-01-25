package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
)

// SSE Data Methods
// These methods extract data for SSE fetchers without HTTP handling.
// Each method returns the same data structure as the corresponding REST API endpoint.

// GetDAGDetailsData returns DAG details for SSE.
// Identifier format: "fileName"
func (a *API) GetDAGDetailsData(ctx context.Context, fileName string) (any, error) {
	dag, err := a.dagStore.GetDetails(ctx, fileName, spec.WithAllowBuildErrors())
	if err != nil {
		return nil, fmt.Errorf("DAG %s not found", fileName)
	}

	dagStatus, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest status for DAG %s", fileName)
	}

	details := toDAGDetails(dag)

	var localDAGs []api.LocalDag
	for _, localDAG := range dag.LocalDAGs {
		localDAGs = append(localDAGs, toLocalDAG(localDAG))
	}

	sort.Slice(localDAGs, func(i, j int) bool {
		return strings.Compare(localDAGs[i].Name, localDAGs[j].Name) <= 0
	})

	var errs []string
	if len(dag.BuildErrors) > 0 {
		for _, buildErr := range dag.BuildErrors {
			errs = append(errs, buildErr.Error())
		}
	}

	return api.GetDAGDetails200JSONResponse{
		Dag:          details,
		LatestDAGRun: ToDAGRunDetails(dagStatus),
		Suspended:    a.dagStore.IsSuspended(ctx, fileName),
		LocalDags:    localDAGs,
		Errors:       errs,
	}, nil
}

// GetDAGRunDetailsData returns DAG run details for SSE.
// Identifier format: "dagName/dagRunId"
func (a *API) GetDAGRunDetailsData(ctx context.Context, identifier string) (any, error) {
	dagName, dagRunId, ok := strings.Cut(identifier, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}

	if dagRunId == "latest" {
		attempt, err := a.dagRunStore.LatestAttempt(ctx, dagName)
		if err != nil {
			return nil, fmt.Errorf("no dag-runs found for DAG %s", dagName)
		}
		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		return api.GetDAGRunDetails200JSONResponse{
			DagRunDetails: ToDAGRunDetails(*status),
		}, nil
	}

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	return api.GetDAGRunDetails200JSONResponse{
		DagRunDetails: ToDAGRunDetails(*dagStatus),
	}, nil
}

// DAGRunLogsResponse represents the response for DAG run logs SSE.
type DAGRunLogsResponse struct {
	SchedulerLog SchedulerLogInfo `json:"schedulerLog"`
	StepLogs     []StepLogInfo    `json:"stepLogs"`
}

// SchedulerLogInfo contains scheduler log metadata.
type SchedulerLogInfo struct {
	Content    string `json:"content"`
	LineCount  int    `json:"lineCount"`
	TotalLines int    `json:"totalLines"`
	HasMore    bool   `json:"hasMore"`
}

// StepLogInfo contains step log metadata.
type StepLogInfo struct {
	StepName    string         `json:"stepName"`
	Status      api.NodeStatus `json:"status"`
	StatusLabel string         `json:"statusLabel"`
	StartedAt   string         `json:"startedAt"`
	FinishedAt  string         `json:"finishedAt"`
	HasStdout   bool           `json:"hasStdout"`
	HasStderr   bool           `json:"hasStderr"`
}

// GetDAGRunLogsData returns DAG run logs for SSE.
// Identifier format: "dagName/dagRunId"
func (a *API) GetDAGRunLogsData(ctx context.Context, identifier string) (any, error) {
	dagName, dagRunId, ok := strings.Cut(identifier, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	// Read scheduler log with default limits
	options := fileutil.LogReadOptions{
		Tail:     500, // Last 500 lines by default
		Encoding: a.logEncodingCharset,
	}

	content, lineCount, totalLines, hasMore, _, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("error reading scheduler log: %w", err)
	}

	schedulerLog := SchedulerLogInfo{
		Content:    content,
		LineCount:  lineCount,
		TotalLines: totalLines,
		HasMore:    hasMore,
	}

	// Build step logs info
	stepLogs := make([]StepLogInfo, 0, len(dagStatus.Nodes))
	for _, node := range dagStatus.Nodes {
		stepLog := StepLogInfo{
			StepName:    node.Step.Name,
			Status:      api.NodeStatus(node.Status),
			StatusLabel: node.Status.String(),
			StartedAt:   node.StartedAt,
			FinishedAt:  node.FinishedAt,
			HasStdout:   node.Stdout != "" && fileExists(node.Stdout),
			HasStderr:   node.Stderr != "" && fileExists(node.Stderr),
		}
		stepLogs = append(stepLogs, stepLog)
	}

	return DAGRunLogsResponse{
		SchedulerLog: schedulerLog,
		StepLogs:     stepLogs,
	}, nil
}

// StepLogResponse represents the response for step log SSE.
type StepLogResponse struct {
	StdoutContent string `json:"stdoutContent"`
	StderrContent string `json:"stderrContent"`
	LineCount     int    `json:"lineCount"`
	TotalLines    int    `json:"totalLines"`
	HasMore       bool   `json:"hasMore"`
}

// GetStepLogData returns step log for SSE.
// Identifier format: "dagName/dagRunId/stepName"
func (a *API) GetStepLogData(ctx context.Context, identifier string) (any, error) {
	parts := strings.SplitN(identifier, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId/stepName')", identifier)
	}
	dagName, dagRunId, stepName := parts[0], parts[1], parts[2]

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	node, err := dagStatus.NodeByName(stepName)
	if err != nil {
		return nil, fmt.Errorf("step %s not found in DAG %s", stepName, dagName)
	}

	options := fileutil.LogReadOptions{
		Tail:     1000, // Last 1000 lines by default
		Encoding: a.logEncodingCharset,
	}

	// Read stdout
	var stdoutContent string
	var lineCount, totalLines int
	var hasMore bool
	if node.Stdout != "" {
		stdoutContent, lineCount, totalLines, hasMore, _, err = fileutil.ReadLogContent(node.Stdout, options)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error reading stdout: %w", err)
		}
	}

	// Read stderr
	var stderrContent string
	if node.Stderr != "" {
		stderrContent, _, _, _, _, err = fileutil.ReadLogContent(node.Stderr, options)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Ignore stderr read errors, just return empty
			stderrContent = ""
		}
	}

	return StepLogResponse{
		StdoutContent: stdoutContent,
		StderrContent: stderrContent,
		LineCount:     lineCount,
		TotalLines:    totalLines,
		HasMore:       hasMore,
	}, nil
}

// GetDAGRunsListData returns DAG runs list for SSE.
// Identifier format: URL query string (e.g., "status=running&name=mydag")
func (a *API) GetDAGRunsListData(ctx context.Context, queryString string) (any, error) {
	params, _ := url.ParseQuery(queryString)

	var opts []exec.ListDAGRunStatusesOption

	if status := params.Get("status"); status != "" {
		if statusInt, err := strconv.Atoi(status); err == nil {
			opts = append(opts, exec.WithStatuses([]core.Status{
				core.Status(statusInt),
			}))
		}
	}
	if fromDate := params.Get("fromDate"); fromDate != "" {
		if ts, err := strconv.ParseInt(fromDate, 10, 64); err == nil {
			dt := exec.NewUTC(time.Unix(ts, 0))
			opts = append(opts, exec.WithFrom(dt))
		}
	}
	if toDate := params.Get("toDate"); toDate != "" {
		if ts, err := strconv.ParseInt(toDate, 10, 64); err == nil {
			dt := exec.NewUTC(time.Unix(ts, 0))
			opts = append(opts, exec.WithTo(dt))
		}
	}
	if name := params.Get("name"); name != "" {
		opts = append(opts, exec.WithName(name))
	}
	if dagRunId := params.Get("dagRunId"); dagRunId != "" {
		opts = append(opts, exec.WithDAGRunID(dagRunId))
	}
	if tags := params.Get("tags"); tags != "" {
		tagList := parseCommaSeparatedTagsString(tags)
		if len(tagList) > 0 {
			opts = append(opts, exec.WithTags(tagList))
		}
	}

	statuses, err := a.dagRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	dagRuns := make([]api.DAGRunSummary, 0, len(statuses))
	for _, status := range statuses {
		dagRuns = append(dagRuns, toDAGRunSummary(*status))
	}

	return api.ListDAGRuns200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

// QueueItemsResponse represents the response for queue items SSE.
type QueueItemsResponse struct {
	Running []api.DAGRunSummary `json:"running"`
	Queued  []api.DAGRunSummary `json:"queued"`
}

// GetQueueItemsData returns queue items for SSE.
// Identifier format: "queueName"
func (a *API) GetQueueItemsData(ctx context.Context, queueName string) (any, error) {
	var running, queued []api.DAGRunSummary

	// Get running items from proc store
	runningByGroup, err := a.procStore.ListAllAlive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list running processes: %w", err)
	}

	runningRefs := runningByGroup[queueName]
	for _, dagRun := range runningRefs {
		attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
		if err != nil {
			continue
		}
		runStatus, err := attempt.ReadStatus(ctx)
		if err != nil {
			continue
		}
		running = append(running, toDAGRunSummary(*runStatus))
	}

	// Get queued items
	queuedItems, err := a.queueStore.List(ctx, queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to list queued items: %w", err)
	}

	for _, queuedItem := range queuedItems {
		dagRunRef, err := queuedItem.Data()
		if err != nil {
			continue
		}
		attempt, err := a.dagRunStore.FindAttempt(ctx, *dagRunRef)
		if err != nil {
			continue
		}
		runStatus, err := attempt.ReadStatus(ctx)
		if err != nil {
			continue
		}
		summary := toDAGRunSummary(*runStatus)
		// Skip running items to avoid duplication
		if summary.Status == api.StatusRunning {
			continue
		}
		queued = append(queued, summary)
	}

	return QueueItemsResponse{
		Running: running,
		Queued:  queued,
	}, nil
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseCommaSeparatedTagsString(tagsParam string) []string {
	if tagsParam == "" {
		return nil
	}

	parts := strings.Split(tagsParam, ",")
	seen := make(map[string]struct{}, len(parts))
	tags := make([]string, 0, len(parts))
	for _, tag := range parts {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = struct{}{}
				tags = append(tags, normalized)
			}
		}
	}
	return tags
}
