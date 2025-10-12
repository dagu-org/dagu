package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/frontend/metrics"
)

// GetHealth implements api.StrictServerInterface.
func (a *API) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return &api.GetHealth200JSONResponse{
		Status:    api.HealthResponseStatusHealthy,
		Version:   config.Version,
		Uptime:    int(metrics.GetUptime()),
		Timestamp: stringutil.FormatTime(time.Now()),
	}, nil
}
