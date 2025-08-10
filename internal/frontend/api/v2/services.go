package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// GetSchedulerStatus returns the status of all registered scheduler instances
func (a *API) GetSchedulerStatus(ctx context.Context, _ api.GetSchedulerStatusRequestObject) (api.GetSchedulerStatusResponseObject, error) {
	logger.Info(ctx, "GetSchedulerStatus called")

	schedulers := []api.SchedulerInstance{}

	// Check if service registry is available
	if a.serviceRegistry == nil {
		return api.GetSchedulerStatusdefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Service registry not configured",
			},
			StatusCode: 500,
		}, nil
	}

	// Get all scheduler instances from service registry
	members, err := a.serviceRegistry.GetServiceMembers(ctx, models.ServiceNameScheduler)
	if err != nil {
		logger.Error(ctx, "Failed to get scheduler members from service registry", "err", err)
		return api.GetSchedulerStatusdefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Failed to retrieve scheduler instances",
			},
			StatusCode: 500,
		}, nil
	}

	// Convert members to API response
	for _, member := range members {
		status := api.SchedulerInstanceStatusUnknown
		switch member.Status {
		case models.ServiceStatusActive:
			status = api.SchedulerInstanceStatusActive
		case models.ServiceStatusInactive:
			status = api.SchedulerInstanceStatusInactive
		}

		schedulers = append(schedulers, api.SchedulerInstance{
			InstanceId: member.ID,
			Host:       member.Host,
			Status:     status,
			StartedAt:  member.StartedAt.Format(time.RFC3339),
		})
	}

	return api.GetSchedulerStatus200JSONResponse{
		Schedulers: schedulers,
	}, nil
}
