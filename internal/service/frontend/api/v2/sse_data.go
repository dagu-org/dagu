package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// SSE Data Methods
// These methods extract data for SSE fetchers without HTTP handling.
// Each method returns the same data structure as the corresponding REST API endpoint.

// GetDAGDetailsData returns DAG details for SSE.
// Identifier format: "fileName"
func (a *API) GetDAGDetailsData(ctx context.Context, fileName string) (any, error) {
	return a.getDAGDetailsData(ctx, fileName)
}

// GetDAGRunDetailsData returns DAG run details for SSE.
// Identifier format: "dagName/dagRunId"
func (a *API) GetDAGRunDetailsData(ctx context.Context, identifier string) (any, error) {
	dagName, dagRunId, ok := strings.Cut(identifier, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}
	return a.getDAGRunDetailsData(ctx, dagName, dagRunId)
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
// Identifier format: "dagName/dagRunId" or "dagName/dagRunId?tail=N"
func (a *API) GetDAGRunLogsData(ctx context.Context, identifier string) (any, error) {
	// Parse query params if present
	pathPart := identifier
	var queryParams url.Values
	if idx := strings.Index(identifier, "?"); idx != -1 {
		pathPart = identifier[:idx]
		queryParams, _ = url.ParseQuery(identifier[idx+1:])
	}

	dagName, dagRunId, ok := strings.Cut(pathPart, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	// Parse tail parameter with bounds validation (100-10000, default 500)
	tail := 500
	if queryParams != nil {
		if tailStr := queryParams.Get("tail"); tailStr != "" {
			if parsed, err := strconv.Atoi(tailStr); err == nil {
				if parsed < 100 {
					tail = 100
				} else if parsed > 10000 {
					tail = 10000
				} else {
					tail = parsed
				}
			}
		}
	}

	options := fileutil.LogReadOptions{
		Tail:     tail,
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
	opts := a.buildDAGRunsListOptions(params)

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

// buildDAGRunsListOptions constructs filter options from URL query parameters.
func (a *API) buildDAGRunsListOptions(params url.Values) []exec.ListDAGRunStatusesOption {
	var opts []exec.ListDAGRunStatusesOption

	if status := params.Get("status"); status != "" {
		if statusInt, err := strconv.Atoi(status); err == nil {
			opts = append(opts, exec.WithStatuses([]core.Status{core.Status(statusInt)}))
		}
	}
	if fromDate := params.Get("fromDate"); fromDate != "" {
		if ts, err := strconv.ParseInt(fromDate, 10, 64); err == nil {
			opts = append(opts, exec.WithFrom(exec.NewUTC(time.Unix(ts, 0))))
		}
	}
	if toDate := params.Get("toDate"); toDate != "" {
		if ts, err := strconv.ParseInt(toDate, 10, 64); err == nil {
			opts = append(opts, exec.WithTo(exec.NewUTC(time.Unix(ts, 0))))
		}
	}
	if name := params.Get("name"); name != "" {
		opts = append(opts, exec.WithName(name))
	}
	if dagRunId := params.Get("dagRunId"); dagRunId != "" {
		opts = append(opts, exec.WithDAGRunID(dagRunId))
	}
	if tags := params.Get("tags"); tags != "" {
		if tagList := parseCommaSeparatedTags(&tags); len(tagList) > 0 {
			opts = append(opts, exec.WithTags(tagList))
		}
	}

	return opts
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

// GetQueuesListData returns queue list for SSE.
// Identifier format: URL query string (ignored for now)
func (a *API) GetQueuesListData(ctx context.Context, _ string) (any, error) {
	response, err := a.ListQueues(ctx, api.ListQueuesRequestObject{})
	if err != nil {
		return nil, fmt.Errorf("error listing queues: %w", err)
	}
	return response, nil
}

// GetDAGsListData returns DAGs list for SSE.
// Identifier format: URL query string (e.g., "page=1&perPage=100&name=mydag")
func (a *API) GetDAGsListData(ctx context.Context, queryString string) (any, error) {
	params, _ := url.ParseQuery(queryString)
	listOpts := a.buildDAGsListOptions(params)

	result, errList, err := a.dagStore.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	dagFiles := make([]api.DAGFile, 0, len(result.Items))
	for _, item := range result.Items {
		dagFile, itemErr := a.buildDAGFile(ctx, item)
		if itemErr != nil {
			errList = append(errList, itemErr.Error())
		}
		dagFiles = append(dagFiles, dagFile)
	}

	return api.ListDAGs200JSONResponse{
		Dags:       dagFiles,
		Errors:     errList,
		Pagination: toPagination(result),
	}, nil
}

// buildDAGsListOptions constructs ListDAGsOptions from URL query parameters.
func (a *API) buildDAGsListOptions(params url.Values) exec.ListDAGsOptions {
	page := parseIntWithDefault(params.Get("page"), 1)
	perPage := parseIntWithDefault(params.Get("perPage"), 100)

	sortField := params.Get("sort")
	if sortField == "" {
		sortField = "name"
	}
	sortOrder := params.Get("order")
	if sortOrder == "" {
		sortOrder = "asc"
	}

	var tags []string
	if tagsParam := params.Get("tags"); tagsParam != "" {
		tags = parseCommaSeparatedTags(&tagsParam)
	}

	pg := exec.NewPaginator(page, perPage)
	return exec.ListDAGsOptions{
		Paginator: &pg,
		Name:      params.Get("name"),
		Tags:      tags,
		Sort:      sortField,
		Order:     sortOrder,
	}
}

// buildDAGFile constructs a DAGFile from a DAG item.
func (a *API) buildDAGFile(ctx context.Context, item *core.DAG) (api.DAGFile, error) {
	dagStatus, err := a.dagRunMgr.GetLatestStatus(ctx, item)

	return api.DAGFile{
		FileName:     item.FileName(),
		LatestDAGRun: toDAGRunSummary(dagStatus),
		Suspended:    a.dagStore.IsSuspended(ctx, item.FileName()),
		Dag:          toDAG(item),
		Errors:       extractBuildErrors(item.BuildErrors),
	}, err
}

// parseIntWithDefault parses an integer string, returning defaultVal if parsing fails or value is <= 0.
func parseIntWithDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return defaultVal
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
