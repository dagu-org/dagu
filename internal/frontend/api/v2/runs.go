package api

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/api/v2"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// GetRunLog implements api.StrictServerInterface.
func (a *API) GetRunLog(ctx context.Context, request api.GetRunLogRequestObject) (api.GetRunLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.client.GetStatus(ctx, dagName, requestId)
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

// GetStepLog implements api.StrictServerInterface.
func (a *API) GetStepLog(ctx context.Context, request api.GetStepLogRequestObject) (api.GetStepLogResponseObject, error) {
	dagName := request.DagName
	requestId := request.RequestId

	status, err := a.client.GetStatus(ctx, dagName, requestId)
	if err != nil {
		return api.GetStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("request ID %s not found for DAG %s", requestId, dagName),
		}, nil
	}

	node, err := status.NodeByName(request.StepName)
	if err != nil {
		return api.GetStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	content, err := a.readFileContent(ctx, node.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetStepLog200JSONResponse{
		Content: string(content),
	}, nil
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
