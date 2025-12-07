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
	if a.resourceService == nil {
		return api.GetResourceHistorydefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Resource service not available",
			},
			StatusCode: 500,
		}, nil
	}

	// Default to 1 hour, capped at retention period
	maxDuration := a.config.Monitoring.Retention
	duration := time.Hour
	if request.Params.Duration != nil {
		if d, err := time.ParseDuration(*request.Params.Duration); err == nil {
			duration = d
		} else {
			logger.Warn(ctx, "Invalid duration parameter", tag.String("duration", *request.Params.Duration))
		}
	}

	// Validate bounds: must be positive and not exceed retention
	if duration <= 0 {
		duration = time.Hour
	}
	if duration > maxDuration {
		duration = maxDuration
	}

	history := a.resourceService.GetHistory(duration)

	cpu := convertMetrics(history.CPU)
	mem := convertMetrics(history.Memory)
	disk := convertMetrics(history.Disk)
	load := convertMetrics(history.Load)

	return api.GetResourceHistory200JSONResponse{
		Cpu:    &cpu,
		Memory: &mem,
		Disk:   &disk,
		Load:   &load,
	}, nil
}

func convertMetrics(points []resource.MetricPoint) []api.MetricPoint {
	result := make([]api.MetricPoint, len(points))
	for i := range points {
		result[i] = api.MetricPoint{
			Timestamp: points[i].Timestamp,
			Value:     points[i].Value,
		}
	}
	return result
}
