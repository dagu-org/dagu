package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"
)

func (a *API) ListDAGRuns(ctx context.Context, request api.ListDAGRunsRequestObject) (api.ListDAGRunsResponseObject, error) {
	var opts []models.ListDAGRunStatusesOption
	if request.Params.Status != nil {
		opts = append(opts, models.WithStatuses([]status.Status{
			status.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := models.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, models.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := models.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, models.WithTo(dt))
	}
	if request.Params.Name != nil {
		opts = append(opts, models.WithName(*request.Params.Name))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, models.WithDAGRunID(*request.Params.DagRunId))
	}

	dagRuns, err := a.listDAGRuns(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	return api.ListDAGRuns200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

func (a *API) ListDAGRunsByName(ctx context.Context, request api.ListDAGRunsByNameRequestObject) (api.ListDAGRunsByNameResponseObject, error) {
	opts := []models.ListDAGRunStatusesOption{
		models.WithExactName(request.Name),
	}

	if request.Params.Status != nil {
		opts = append(opts, models.WithStatuses([]status.Status{
			status.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := models.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, models.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := models.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, models.WithTo(dt))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, models.WithDAGRunID(*request.Params.DagRunId))
	}

	dagRuns, err := a.listDAGRuns(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	return api.ListDAGRunsByName200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

func (a *API) listDAGRuns(ctx context.Context, opts []models.ListDAGRunStatusesOption) ([]api.DAGRunSummary, error) {
	statuses, err := a.dagRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}
	var dagRuns []api.DAGRunSummary
	for _, status := range statuses {
		dagRuns = append(dagRuns, toDAGRunSummary(*status))
	}
	return dagRuns, nil
}

func (a *API) GetDAGRunLog(ctx context.Context, request api.GetDAGRunLogRequestObject) (api.GetDAGRunLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := digraph.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	return api.GetDAGRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) GetDAGRunStepLog(ctx context.Context, request api.GetDAGRunStepLogRequestObject) (api.GetDAGRunStepLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := digraph.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return api.GetDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	var logFile = node.Stdout
	if *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(logFile, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Stdout, err)
	}

	return api.GetDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) UpdateDAGRunStepStatus(ctx context.Context, request api.UpdateDAGRunStepStatusRequestObject) (api.UpdateDAGRunStepStatusResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	ref := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.UpdateDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}
	if dagStatus.Status == status.Running {
		return &api.UpdateDAGRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("dag-run ID %s for DAG %s is still running", request.DagRunId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	root := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	if err := a.dagRunMgr.UpdateStatus(ctx, root, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateDAGRunStepStatus200Response{}, nil
}

// GetDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetDAGRunDetails(ctx context.Context, request api.GetDAGRunDetailsRequestObject) (api.GetDAGRunDetailsResponseObject, error) {
	ref := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.GetDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}
	return &api.GetDAGRunDetails200JSONResponse{
		DagRunDetails: toDAGRunDetails(*dagStatus),
	}, nil
}

// GetChildDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetChildDAGRunDetails(ctx context.Context, request api.GetChildDAGRunDetailsRequestObject) (api.GetChildDAGRunDetailsResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindChildDAGRunStatus(ctx, root, request.ChildDAGRunId)
	if err != nil {
		return &api.GetChildDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child dag-run ID %s not found for DAG %s", request.ChildDAGRunId, request.Name),
		}, nil
	}
	return &api.GetChildDAGRunDetails200JSONResponse{
		DagRunDetails: toDAGRunDetails(*dagStatus),
	}, nil
}

// GetChildDAGRunLog implements api.StrictServerInterface.
func (a *API) GetChildDAGRunLog(ctx context.Context, request api.GetChildDAGRunLogRequestObject) (api.GetChildDAGRunLogResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindChildDAGRunStatus(ctx, root, request.ChildDAGRunId)
	if err != nil {
		return &api.GetChildDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child dag-run ID %s not found for DAG %s", request.ChildDAGRunId, request.Name),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	return &api.GetChildDAGRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// GetChildDAGRunStepLog implements api.StrictServerInterface.
func (a *API) GetChildDAGRunStepLog(ctx context.Context, request api.GetChildDAGRunStepLogRequestObject) (api.GetChildDAGRunStepLogResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindChildDAGRunStatus(ctx, root, request.ChildDAGRunId)
	if err != nil {
		return &api.GetChildDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child dag-run ID %s not found for DAG %s", request.ChildDAGRunId, request.Name),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return &api.GetChildDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	var logFile = node.Stdout
	if *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(logFile, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Stdout, err)
	}

	return &api.GetChildDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// UpdateChildDAGRunStepStatus implements api.StrictServerInterface.
func (a *API) UpdateChildDAGRunStepStatus(ctx context.Context, request api.UpdateChildDAGRunStepStatusRequestObject) (api.UpdateChildDAGRunStepStatusResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	root := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindChildDAGRunStatus(ctx, root, request.ChildDAGRunId)
	if err != nil {
		return &api.UpdateChildDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child dag-run ID %s not found for DAG %s", request.ChildDAGRunId, request.Name),
		}, nil
	}
	if dagStatus.Status == status.Running {
		return &api.UpdateChildDAGRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("dag-run ID %s for DAG %s is still running", request.DagRunId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateChildDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	if err := a.dagRunMgr.UpdateStatus(ctx, root, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateChildDAGRunStepStatus200Response{}, nil
}

var nodeStatusMapping = map[api.NodeStatus]status.NodeStatus{
	api.NodeStatusNotStarted: status.NodeNone,
	api.NodeStatusRunning:    status.NodeRunning,
	api.NodeStatusFailed:     status.NodeError,
	api.NodeStatusCancelled:  status.NodeCancel,
	api.NodeStatusSuccess:    status.NodeSuccess,
	api.NodeStatusSkipped:    status.NodeSkipped,
}

func (a *API) RetryDAGRun(ctx context.Context, request api.RetryDAGRunRequestObject) (api.RetryDAGRunResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	attempt, err := a.dagRunStore.FindAttempt(ctx, digraph.NewDAGRunRef(request.Name, request.DagRunId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	if request.Body.StepName != nil && *request.Body.StepName != "" {
		if err := a.dagRunMgr.RetryDAGStep(ctx, dag, request.Body.DagRunId, *request.Body.StepName); err != nil {
			return nil, fmt.Errorf("error retrying DAG step: %w", err)
		}
		return api.RetryDAGRun200Response{}, nil
	}

	if err := a.dagRunMgr.RetryDAGRun(ctx, dag, request.Body.DagRunId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGRun200Response{}, nil
}

func (a *API) TerminateDAGRun(ctx context.Context, request api.TerminateDAGRunRequestObject) (api.TerminateDAGRunResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	attempt, err := a.dagRunStore.FindAttempt(ctx, digraph.NewDAGRunRef(request.Name, request.DagRunId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	dagStatus, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, request.DagRunId)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}

	if dagStatus.Status != status.Running {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is not running",
		}
	}

	if err := a.dagRunMgr.Stop(ctx, dag, dagStatus.DAGRunID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}

	return api.TerminateDAGRun200Response{}, nil
}

func (a *API) DequeueDAGRun(ctx context.Context, request api.DequeueDAGRunRequestObject) (api.DequeueDAGRunResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	dagRun := digraph.NewDAGRunRef(request.Name, request.DagRunId)
	attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	latestStatus, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRun.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if latestStatus.Status != status.Queued {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("DAGRun status is not queued: %s", latestStatus.Status),
		}
	}

	if err := a.dagRunMgr.DequeueDAGRun(ctx, dag, dagRun); err != nil {
		return nil, fmt.Errorf("error dequeueing dag-run: %w", err)
	}

	return api.DequeueDAGRun200Response{}, nil
}
