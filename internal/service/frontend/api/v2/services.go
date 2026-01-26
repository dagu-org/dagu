package api

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/tunnel"
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
	members, err := a.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameScheduler)
	if err != nil {
		logger.Error(ctx, "Failed to get scheduler members from service registry", tag.Error(err))
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
		case exec.ServiceStatusActive:
			status = api.SchedulerInstanceStatusActive
		case exec.ServiceStatusInactive:
			status = api.SchedulerInstanceStatusInactive
		case exec.ServiceStatusUnknown:
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
	members, err := a.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameCoordinator)
	if err != nil {
		logger.Error(ctx, "Failed to get coordinator members from service registry", tag.Error(err))
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
		case exec.ServiceStatusActive:
			status = api.CoordinatorInstanceStatusActive
		case exec.ServiceStatusInactive:
			status = api.CoordinatorInstanceStatusInactive
		case exec.ServiceStatusUnknown:
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

// GetTunnelStatus returns the status of the tunnel service
func (a *API) GetTunnelStatus(ctx context.Context, _ api.GetTunnelStatusRequestObject) (api.GetTunnelStatusResponseObject, error) {
	logger.Info(ctx, "GetTunnelStatus called")

	// Check if tunnel is enabled in config
	if !a.config.Tunnel.Enabled {
		return api.GetTunnelStatus200JSONResponse{
			Enabled: false,
			Status:  api.TunnelStatusResponseStatusDisabled,
		}, nil
	}

	// If no tunnel service is available, return disabled status
	if a.tunnelService == nil {
		return api.GetTunnelStatus200JSONResponse{
			Enabled: true,
			Status:  api.TunnelStatusResponseStatusDisabled,
		}, nil
	}

	// Get tunnel info from service
	info := a.tunnelService.Info()

	// Convert tunnel.Status to API status
	var status api.TunnelStatusResponseStatus
	switch info.Status {
	case tunnel.StatusConnected:
		status = api.TunnelStatusResponseStatusConnected
	case tunnel.StatusConnecting:
		status = api.TunnelStatusResponseStatusConnecting
	case tunnel.StatusReconnecting:
		status = api.TunnelStatusResponseStatusReconnecting
	case tunnel.StatusError:
		status = api.TunnelStatusResponseStatusError
	default:
		status = api.TunnelStatusResponseStatusDisabled
	}

	// Convert provider type
	var provider *api.TunnelStatusResponseProvider
	if info.Provider != "" {
		p := api.TunnelStatusResponseProvider(info.Provider)
		provider = &p
	}

	// Build response
	response := api.GetTunnelStatus200JSONResponse{
		Enabled:   true,
		Status:    status,
		Provider:  provider,
		PublicUrl: ptrOf(info.PublicURL),
		Error:     ptrOf(info.Error),
		Mode:      ptrOf(info.Mode),
		IsPublic:  ptrOf(info.IsPublic),
	}

	// Add startedAt if connected
	if !info.StartedAt.IsZero() {
		startedAt := info.StartedAt.Format(time.RFC3339)
		response.StartedAt = &startedAt
	}

	return response, nil
}
