package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/service/frontend/metrics"
)

func (a *API) GetHealthStatus(_ context.Context, _ api.GetHealthStatusRequestObject) (api.GetHealthStatusResponseObject, error) {
	return &api.GetHealthStatus200JSONResponse{
		Status:    api.HealthResponseStatusHealthy,
		Version:   config.Version,
		Uptime:    int(metrics.GetUptime()),
		Timestamp: stringutil.FormatTime(time.Now()),
	}, nil
}
