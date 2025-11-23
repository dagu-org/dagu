package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
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
	members, err := a.serviceRegistry.GetServiceMembers(ctx, execution.ServiceNameScheduler)
	if err != nil {
		logger.Error(ctx, "Failed to get scheduler members from service registry", tag.Error, err)
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
		var status api.SchedulerInstanceStatus
		switch member.Status {
		case execution.ServiceStatusActive:
			status = api.SchedulerInstanceStatusActive
		case execution.ServiceStatusInactive:
			status = api.SchedulerInstanceStatusInactive
		case execution.ServiceStatusUnknown:
			status = api.SchedulerInstanceStatusUnknown
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

// GetCoordinatorStatus returns the status of all registered coordinator instances
func (a *API) GetCoordinatorStatus(ctx context.Context, _ api.GetCoordinatorStatusRequestObject) (api.GetCoordinatorStatusResponseObject, error) {
	logger.Info(ctx, "GetCoordinatorStatus called")

	coordinators := []api.CoordinatorInstance{}

	// Check if service registry is available
	if a.serviceRegistry == nil {
		return api.GetCoordinatorStatusdefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Service registry not configured",
			},
			StatusCode: 500,
		}, nil
	}

	// Get all coordinator instances from service registry
	members, err := a.serviceRegistry.GetServiceMembers(ctx, execution.ServiceNameCoordinator)
	if err != nil {
		logger.Error(ctx, "Failed to get coordinator members from service registry", tag.Error, err)
		return api.GetCoordinatorStatusdefaultJSONResponse{
			Body: api.Error{
				Code:    api.ErrorCodeInternalError,
				Message: "Failed to retrieve coordinator instances",
			},
			StatusCode: 500,
		}, nil
	}

	// Convert members to API response
	for _, member := range members {
		var status api.CoordinatorInstanceStatus
		switch member.Status {
		case execution.ServiceStatusActive:
			status = api.CoordinatorInstanceStatusActive
		case execution.ServiceStatusInactive:
			status = api.CoordinatorInstanceStatusInactive
		case execution.ServiceStatusUnknown:
			status = api.CoordinatorInstanceStatusUnknown
		}

		coordinators = append(coordinators, api.CoordinatorInstance{
			InstanceId: member.ID,
			Host:       member.Host,
			Port:       member.Port,
			Status:     status,
			StartedAt:  member.StartedAt.Format(time.RFC3339),
		})
	}

	return api.GetCoordinatorStatus200JSONResponse{
		Coordinators: coordinators,
	}, nil
}
