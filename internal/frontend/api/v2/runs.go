package api

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

func (a *API) GetDAGRunLog(ctx context.Context, request api.GetDAGRunLogRequestObject) (api.GetDAGRunLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.client.GetStatus(ctx, dagName, requestId)
	if err != nil {
		return api.GetDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", requestId, dagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, status.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetDAGRunLog200JSONResponse{
		Content: string(content),
	}, nil
}

func (a *API) GetDAGStepLog(ctx context.Context, request api.GetDAGStepLogRequestObject) (api.GetDAGStepLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.client.GetStatus(ctx, dagName, requestId)
	if err != nil {
		return api.GetDAGStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", requestId, dagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return api.GetDAGStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, node.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetDAGStepLog200JSONResponse{
		Content: string(content),
	}, nil
}

func (a *API) UpdateDAGStepStatus(ctx context.Context, request api.UpdateDAGStepStatusRequestObject) (api.UpdateDAGStepStatusResponseObject, error) {
	status, err := a.client.GetStatus(ctx, request.DagName, request.RequestId)
	if err != nil {
		return &api.UpdateDAGStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", request.RequestId, request.DagName),
		}, nil
	}
	if status.Status == scheduler.StatusRunning {
		return &api.UpdateDAGStepStatus400JSONResponse{
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
		return &api.UpdateDAGStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.DagName),
		}, nil
	}

	status.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]
	status.Nodes[idxToUpdate].StatusText = nodeStatusMapping[request.Body.Status].String()

	if err := a.client.UpdateStatus(ctx, request.DagName, *status); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateDAGStepStatus200Response{}, nil
}

func (a *API) readFileContent(ctx context.Context, f string, d *encoding.Decoder) ([]byte, error) {
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
