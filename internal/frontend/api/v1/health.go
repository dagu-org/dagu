package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/frontend/metrics"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// GetHealth implements api.StrictServerInterface.
func (a *API) GetHealth(_ context.Context, request api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return &api.GetHealth200JSONResponse{
		Status:    api.HealthResponseStatusHealthy,
		Version:   build.Version,
		Uptime:    int(metrics.GetUptime()),
		Timestamp: stringutil.FormatTime(time.Now()),
	}, nil
}
