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

	// FIXME: Load DAG from the runs database
	dagWithStatus, err := a.client.GetStatus(ctx, dagName)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	status := dagWithStatus.Status
	if requestId != "latest" {
		s, err := a.client.GetStatusByRequestID(ctx, dagWithStatus.DAG, requestId)
		if err != nil {
			return nil, fmt.Errorf("error getting status by request ID: %w", err)
		}
		status = *s
	}

	content, err := a.readFileContent(ctx, status.Log, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", status.Log, err)
	}

	return api.GetRunLog200JSONResponse{
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
