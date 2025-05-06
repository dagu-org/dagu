package api

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

func (a *API) GetRunLog(ctx context.Context, request api.GetRunLogRequestObject) (api.GetRunLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.runClient.FindByRequestID(ctx, dagName, requestId)
	if err != nil {
		return api.GetRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", requestId, dagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, status.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetRunLog200JSONResponse{
		Content: string(content),
	}, nil
}

func (a *API) GetRunStepLog(ctx context.Context, request api.GetRunStepLogRequestObject) (api.GetRunStepLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.runClient.FindByRequestID(ctx, dagName, requestId)
	if err != nil {
		return api.GetRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", requestId, dagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return api.GetRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, node.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetRunStepLog200JSONResponse{
		Content: string(content),
	}, nil
}

func (a *API) UpdateRunStepStatus(ctx context.Context, request api.UpdateRunStepStatusRequestObject) (api.UpdateRunStepStatusResponseObject, error) {
	status, err := a.runClient.FindByRequestID(ctx, request.DagName, request.RequestId)
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

	rootDAG := digraph.NewRootDAG(request.DagName, request.RequestId)
	if err := a.runClient.UpdateStatus(ctx, rootDAG, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateRunStepStatus200Response{}, nil
}

// GetRunDetails implements api.StrictServerInterface.
func (a *API) GetRunDetails(ctx context.Context, request api.GetRunDetailsRequestObject) (api.GetRunDetailsResponseObject, error) {
	status, err := a.runClient.FindByRequestID(ctx, request.DagName, request.RequestId)
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
	root := digraph.NewRootDAG(request.DagName, request.RequestId)
	status, err := a.runClient.FindBySubRunRequestID(ctx, root, request.SubRunRequestId)
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
	root := digraph.NewRootDAG(request.DagName, request.RequestId)
	status, err := a.runClient.FindBySubRunRequestID(ctx, root, request.SubRunRequestId)
	if err != nil {
		return &api.GetSubRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, status.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return &api.GetSubRunLog200JSONResponse{
		Content: string(content),
	}, nil
}

// GetSubRunStepLog implements api.StrictServerInterface.
func (a *API) GetSubRunStepLog(ctx context.Context, request api.GetSubRunStepLogRequestObject) (api.GetSubRunStepLogResponseObject, error) {
	root := digraph.NewRootDAG(request.DagName, request.RequestId)
	status, err := a.runClient.FindBySubRunRequestID(ctx, root, request.SubRunRequestId)
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

	content, err := a.readFileContent(ctx, node.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return &api.GetSubRunStepLog200JSONResponse{
		Content: string(content),
	}, nil
}

// UpdateSubRunStepStatus implements api.StrictServerInterface.
func (a *API) UpdateSubRunStepStatus(ctx context.Context, request api.UpdateSubRunStepStatusRequestObject) (api.UpdateSubRunStepStatusResponseObject, error) {
	root := digraph.NewRootDAG(request.DagName, request.RequestId)
	status, err := a.runClient.FindBySubRunRequestID(ctx, root, request.SubRunRequestId)
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

	if err := a.runClient.UpdateStatus(ctx, root, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateSubRunStepStatus200Response{}, nil
}

func (a *API) readFileContent(_ context.Context, f string, d *encoding.Decoder) ([]byte, error) {
	if d == nil {
		return os.ReadFile(f) //nolint:gosec
	}

	r, err := os.Open(f) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", f, err)
	}
	defer func() {
		_ = r.Close()
	}()
	tr := transform.NewReader(r, d)
	ret, err := io.ReadAll(tr)
	return ret, err
}

var nodeStatusMapping = map[api.NodeStatus]scheduler.NodeStatus{
	api.NodeStatusNotStarted: scheduler.NodeStatusNone,
	api.NodeStatusRunning:    scheduler.NodeStatusRunning,
	api.NodeStatusFailed:     scheduler.NodeStatusError,
	api.NodeStatusCancelled:  scheduler.NodeStatusCancel,
	api.NodeStatusSuccess:    scheduler.NodeStatusSuccess,
	api.NodeStatusSkipped:    scheduler.NodeStatusSkipped,
}
