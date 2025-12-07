package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/service/resource"
)

func (a *API) GetResourceHistory(ctx context.Context, request api.GetResourceHistoryRequestObject) (api.GetResourceHistoryResponseObject, error) {
	logger.Info(ctx, "GetResourceHistory called")

	if a.resourceService == nil {
		return api.GetResourceHistorydefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Resource service not available",
			},
			StatusCode: 500,
		}, nil
	}

	duration := time.Hour // Default
	if request.Params.Duration != nil {
		if d, err := time.ParseDuration(*request.Params.Duration); err == nil {
			duration = d
		} else {
			logger.Warn(ctx, "Invalid duration parameter", tag.String("duration", *request.Params.Duration))
		}
	}

	history := a.resourceService.GetHistory(duration)

	return api.GetResourceHistory200JSONResponse{
		Cpu:    convertMetrics(history.CPU),
		Memory: convertMetrics(history.Memory),
		Disk:   convertMetrics(history.Disk),
		Load:   convertMetrics(history.Load),
	}, nil
}

func convertMetrics(points []resource.MetricPoint) *[]api.MetricPoint {
	result := make([]api.MetricPoint, len(points))
	for i := range points {
		result[i] = api.MetricPoint{
			Timestamp: &points[i].Timestamp,
			Value:     &points[i].Value,
		}
	}
	return &result
}
