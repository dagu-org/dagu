package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"
)

func (a *API) ListWorkflows(ctx context.Context, request api.ListWorkflowsRequestObject) (api.ListWorkflowsResponseObject, error) {
	var opts []models.ListDAGRunStatusesOption
	if request.Params.Status != nil {
		opts = append(opts, models.WithStatuses([]scheduler.Status{
			scheduler.Status(*request.Params.Status),
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
	if request.Params.WorkflowId != nil {
		opts = append(opts, models.WithDAGRunID(*request.Params.WorkflowId))
	}

	workflows, err := a.listWorkflows(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing workflows: %w", err)
	}

	return api.ListWorkflows200JSONResponse{
		Workflows: workflows,
	}, nil
}

func (a *API) ListWorkflowsByName(ctx context.Context, request api.ListWorkflowsByNameRequestObject) (api.ListWorkflowsByNameResponseObject, error) {
	opts := []models.ListDAGRunStatusesOption{
		models.WithExactName(request.Name),
	}

	if request.Params.Status != nil {
		opts = append(opts, models.WithStatuses([]scheduler.Status{
			scheduler.Status(*request.Params.Status),
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
	if request.Params.WorkflowId != nil {
		opts = append(opts, models.WithDAGRunID(*request.Params.WorkflowId))
	}

	workflows, err := a.listWorkflows(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing workflows: %w", err)
	}

	return api.ListWorkflowsByName200JSONResponse{
		Workflows: workflows,
	}, nil
}

func (a *API) listWorkflows(ctx context.Context, opts []models.ListDAGRunStatusesOption) ([]api.WorkflowSummary, error) {
	statuses, err := a.historyStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing workflows: %w", err)
	}
	var workflows []api.WorkflowSummary
	for _, status := range statuses {
		workflows = append(workflows, toWorkflowSummary(*status))
	}
	return workflows, nil
}

func (a *API) GetWorkflowLog(ctx context.Context, request api.GetWorkflowLogRequestObject) (api.GetWorkflowLogResponseObject, error) {
	dagName := request.Name
	workflowId := request.WorkflowId

	ref := digraph.NewDAGRunRef(dagName, workflowId)
	status, err := a.historyManager.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetWorkflowLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("workflow ID %s not found for DAG %s", workflowId, dagName),
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
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(status.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetWorkflowLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) GetWorkflowStepLog(ctx context.Context, request api.GetWorkflowStepLogRequestObject) (api.GetWorkflowStepLogResponseObject, error) {
	dagName := request.Name
	workflowId := request.WorkflowId

	ref := digraph.NewDAGRunRef(dagName, workflowId)
	status, err := a.historyManager.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetWorkflowStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("workflow ID %s not found for DAG %s", workflowId, dagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return api.GetWorkflowStepLog404JSONResponse{
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

	return api.GetWorkflowStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) UpdateWorkflowStepStatus(ctx context.Context, request api.UpdateWorkflowStepStatusRequestObject) (api.UpdateWorkflowStepStatusResponseObject, error) {
	ref := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.UpdateWorkflowStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("workflow ID %s not found for DAG %s", request.WorkflowId, request.Name),
		}, nil
	}
	if status.Status == scheduler.StatusRunning {
		return &api.UpdateWorkflowStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("workflow ID %s for DAG %s is still running", request.WorkflowId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range status.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateWorkflowStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	status.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	root := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	if err := a.historyManager.UpdateStatus(ctx, root, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateWorkflowStepStatus200Response{}, nil
}

// GetWorkflowDetails implements api.StrictServerInterface.
func (a *API) GetWorkflowDetails(ctx context.Context, request api.GetWorkflowDetailsRequestObject) (api.GetWorkflowDetailsResponseObject, error) {
	ref := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.GetWorkflowDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("workflow ID %s not found for DAG %s", request.WorkflowId, request.Name),
		}, nil
	}
	return &api.GetWorkflowDetails200JSONResponse{
		WorkflowDetails: toWorkflowDetails(*status),
	}, nil
}

// GetChildWorkflowDetails implements api.StrictServerInterface.
func (a *API) GetChildWorkflowDetails(ctx context.Context, request api.GetChildWorkflowDetailsRequestObject) (api.GetChildWorkflowDetailsResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.FindChildDAGRunStatus(ctx, root, request.ChildWorkflowId)
	if err != nil {
		return &api.GetChildWorkflowDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child workflow ID %s not found for DAG %s", request.ChildWorkflowId, request.Name),
		}, nil
	}
	return &api.GetChildWorkflowDetails200JSONResponse{
		WorkflowDetails: toWorkflowDetails(*status),
	}, nil
}

// GetChildWorkflowLog implements api.StrictServerInterface.
func (a *API) GetChildWorkflowLog(ctx context.Context, request api.GetChildWorkflowLogRequestObject) (api.GetChildWorkflowLogResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.FindChildDAGRunStatus(ctx, root, request.ChildWorkflowId)
	if err != nil {
		return &api.GetChildWorkflowLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child workflow ID %s not found for DAG %s", request.ChildWorkflowId, request.Name),
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
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(status.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return &api.GetChildWorkflowLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// GetChildWorkflowStepLog implements api.StrictServerInterface.
func (a *API) GetChildWorkflowStepLog(ctx context.Context, request api.GetChildWorkflowStepLogRequestObject) (api.GetChildWorkflowStepLogResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.FindChildDAGRunStatus(ctx, root, request.ChildWorkflowId)
	if err != nil {
		return &api.GetChildWorkflowStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child workflow ID %s not found for DAG %s", request.ChildWorkflowId, request.Name),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return &api.GetChildWorkflowStepLog404JSONResponse{
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

	return &api.GetChildWorkflowStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// UpdateChildWorkflowStepStatus implements api.StrictServerInterface.
func (a *API) UpdateChildWorkflowStepStatus(ctx context.Context, request api.UpdateChildWorkflowStepStatusRequestObject) (api.UpdateChildWorkflowStepStatusResponseObject, error) {
	root := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	status, err := a.historyManager.FindChildDAGRunStatus(ctx, root, request.ChildWorkflowId)
	if err != nil {
		return &api.UpdateChildWorkflowStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("child workflow ID %s not found for DAG %s", request.ChildWorkflowId, request.Name),
		}, nil
	}
	if status.Status == scheduler.StatusRunning {
		return &api.UpdateChildWorkflowStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("workflow ID %s for DAG %s is still running", request.WorkflowId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range status.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateChildWorkflowStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	status.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	if err := a.historyManager.UpdateStatus(ctx, root, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateChildWorkflowStepStatus200Response{}, nil
}

var nodeStatusMapping = map[api.NodeStatus]scheduler.NodeStatus{
	api.NodeStatusNotStarted: scheduler.NodeStatusNone,
	api.NodeStatusRunning:    scheduler.NodeStatusRunning,
	api.NodeStatusFailed:     scheduler.NodeStatusError,
	api.NodeStatusCancelled:  scheduler.NodeStatusCancel,
	api.NodeStatusSuccess:    scheduler.NodeStatusSuccess,
	api.NodeStatusSkipped:    scheduler.NodeStatusSkipped,
}

func (a *API) RetryWorkflow(ctx context.Context, request api.RetryWorkflowRequestObject) (api.RetryWorkflowResponseObject, error) {
	attempt, err := a.historyStore.FindAttempt(ctx, digraph.NewDAGRunRef(request.Name, request.WorkflowId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("workflow ID %s not found for DAG %s", request.WorkflowId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	if err := a.historyManager.RetryDAGRun(ctx, dag, request.Body.WorkflowId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryWorkflow200Response{}, nil
}

func (a *API) TerminateWorkflow(ctx context.Context, request api.TerminateWorkflowRequestObject) (api.TerminateWorkflowResponseObject, error) {
	attempt, err := a.historyStore.FindAttempt(ctx, digraph.NewDAGRunRef(request.Name, request.WorkflowId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("workflow ID %s not found for DAG %s", request.WorkflowId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	status, err := a.historyManager.GetCurrentStatus(ctx, dag, request.WorkflowId)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}

	if status.Status != scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is not running",
		}
	}

	if err := a.historyManager.Stop(ctx, dag, status.DAGRunID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}

	return api.TerminateWorkflow200Response{}, nil
}

func (a *API) DequeueWorkflow(ctx context.Context, request api.DequeueWorkflowRequestObject) (api.DequeueWorkflowResponseObject, error) {
	workflow := digraph.NewDAGRunRef(request.Name, request.WorkflowId)
	attempt, err := a.historyStore.FindAttempt(ctx, workflow)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("workflow ID %s not found for DAG %s", request.WorkflowId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	latestStatus, err := a.historyManager.GetCurrentStatus(ctx, dag, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if latestStatus.Status != scheduler.StatusQueued {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("Workflow status is not queued: %s", latestStatus.Status),
		}
	}

	if err := a.historyManager.DequeueDAGRun(ctx, workflow); err != nil {
		return nil, fmt.Errorf("error dequeueing workflow: %w", err)
	}

	return api.DequeueWorkflow200Response{}, nil
}
