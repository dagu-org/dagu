package api

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
)

func (a *API) GetRunLog(ctx context.Context, request api.GetRunLogRequestObject) (api.GetRunLogResponseObject, error) {
	dagName := request.DagName
	reqID := request.RequestId

	status, err := a.historyManager.FindByReqID(ctx, dagName, reqID)
	if err != nil {
		return api.GetRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", reqID, dagName),
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

	return api.GetRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) GetRunStepLog(ctx context.Context, request api.GetRunStepLogRequestObject) (api.GetRunStepLogResponseObject, error) {
	dagName := request.DagName
	reqID := request.RequestId

	status, err := a.historyManager.FindByReqID(ctx, dagName, reqID)
	if err != nil {
		return api.GetRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", reqID, dagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return api.GetRunStepLog404JSONResponse{
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

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(node.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return api.GetRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) UpdateRunStepStatus(ctx context.Context, request api.UpdateRunStepStatusRequestObject) (api.UpdateRunStepStatusResponseObject, error) {
	status, err := a.historyManager.FindByReqID(ctx, request.DagName, request.RequestId)
	if err != nil {
		return &api.UpdateRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}
	if status.Status == scheduler.StatusRunning {
		return &api.UpdateRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("request ID %s for DAG %s is still running", request.RequestId, request.DagName),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range status.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.DagName),
		}, nil
	}

	status.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	rootRun := digraph.NewRootRun(request.DagName, request.RequestId)
	if err := a.historyManager.UpdateStatus(ctx, rootRun, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateRunStepStatus200Response{}, nil
}

// GetRunDetails implements api.StrictServerInterface.
func (a *API) GetRunDetails(ctx context.Context, request api.GetRunDetailsRequestObject) (api.GetRunDetailsResponseObject, error) {
	status, err := a.historyManager.FindByReqID(ctx, request.DagName, request.RequestId)
	if err != nil {
		return &api.GetRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}
	return &api.GetRunDetails200JSONResponse{
		RunDetails: toRunDetails(*status),
	}, nil
}

// GetSubRunDetails implements api.StrictServerInterface.
func (a *API) GetSubRunDetails(ctx context.Context, request api.GetSubRunDetailsRequestObject) (api.GetSubRunDetailsResponseObject, error) {
	root := digraph.NewRootRun(request.DagName, request.RequestId)
	status, err := a.historyManager.FindBySubRunReqID(ctx, root, request.SubRunRequestId)
	if err != nil {
		return &api.GetSubRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}
	return &api.GetSubRunDetails200JSONResponse{
		RunDetails: toRunDetails(*status),
	}, nil
}

// GetSubRunLog implements api.StrictServerInterface.
func (a *API) GetSubRunLog(ctx context.Context, request api.GetSubRunLogRequestObject) (api.GetSubRunLogResponseObject, error) {
	root := digraph.NewRootRun(request.DagName, request.RequestId)
	status, err := a.historyManager.FindBySubRunReqID(ctx, root, request.SubRunRequestId)
	if err != nil {
		return &api.GetSubRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
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

	return &api.GetSubRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// GetSubRunStepLog implements api.StrictServerInterface.
func (a *API) GetSubRunStepLog(ctx context.Context, request api.GetSubRunStepLogRequestObject) (api.GetSubRunStepLogResponseObject, error) {
	root := digraph.NewRootRun(request.DagName, request.RequestId)
	status, err := a.historyManager.FindBySubRunReqID(ctx, root, request.SubRunRequestId)
	if err != nil {
		return &api.GetSubRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return &api.GetSubRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.DagName),
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
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(node.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return &api.GetSubRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// UpdateSubRunStepStatus implements api.StrictServerInterface.
func (a *API) UpdateSubRunStepStatus(ctx context.Context, request api.UpdateSubRunStepStatusRequestObject) (api.UpdateSubRunStepStatusResponseObject, error) {
	root := digraph.NewRootRun(request.DagName, request.RequestId)
	status, err := a.historyManager.FindBySubRunReqID(ctx, root, request.SubRunRequestId)
	if err != nil {
		return &api.UpdateSubRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}
	if status.Status == scheduler.StatusRunning {
		return &api.UpdateSubRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("request ID %s for DAG %s is still running", request.RequestId, request.DagName),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range status.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateSubRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.DagName),
		}, nil
	}

	status.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	if err := a.historyManager.UpdateStatus(ctx, root, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateSubRunStepStatus200Response{}, nil
}

var nodeStatusMapping = map[api.NodeStatus]scheduler.NodeStatus{
	api.NodeStatusNotStarted: scheduler.NodeStatusNone,
	api.NodeStatusRunning:    scheduler.NodeStatusRunning,
	api.NodeStatusFailed:     scheduler.NodeStatusError,
	api.NodeStatusCancelled:  scheduler.NodeStatusCancel,
	api.NodeStatusSuccess:    scheduler.NodeStatusSuccess,
	api.NodeStatusSkipped:    scheduler.NodeStatusSkipped,
}
