package api

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetWorkers implements the getWorkers operation
func (a *API) GetWorkers(ctx context.Context, _ api.GetWorkersRequestObject) (api.GetWorkersResponseObject, error) {
	logger.Info(ctx, "GetWorkers called")

	errors := []string{}
	workers := []api.Worker{}

	// Check if coordinator client factory is available
	if a.coordinatorClientFactory == nil {
		errors = append(errors, "Coordinator service not configured")
		return api.GetWorkers200JSONResponse{
			Workers: workers,
			Errors:  errors,
		}, nil
	}

	// Build coordinator client on demand
	coordinatorClient, err := a.coordinatorClientFactory.Build(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to build coordinator client", "err", err)
		errors = append(errors, fmt.Sprintf("Failed to connect to coordinator: %v", err))
		return api.GetWorkers200JSONResponse{
			Workers: workers,
			Errors:  errors,
		}, nil
	}
	defer func() {
		if err := coordinatorClient.Close(); err != nil {
			logger.Error(ctx, "Failed to close coordinator client", "err", err)
		}
	}()

	// Call the coordinator to get workers
	grpcClient := coordinatorClient.GetGRPCClient()
	resp, err := grpcClient.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
	if err != nil {
		// Check if it's a connection error
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unavailable {
				return api.GetWorkers503JSONResponse{
					Message: "Coordinator service unavailable",
				}, nil
			}
		}
		errors = append(errors, err.Error())
		return api.GetWorkers200JSONResponse{
			Workers: workers,
			Errors:  errors,
		}, nil
	}

	// Transform the response
	for _, w := range resp.Workers {
		// Transform protobuf health status to API health status
		var healthStatus api.WorkerHealthStatus
		switch w.HealthStatus {
		case coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY:
			healthStatus = api.WorkerHealthStatusHealthy
		case coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING:
			healthStatus = api.WorkerHealthStatusWarning
		case coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY:
			healthStatus = api.WorkerHealthStatusUnhealthy
		case coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNSPECIFIED:
			healthStatus = api.WorkerHealthStatusHealthy // Default to healthy if unspecified
		default:
			healthStatus = api.WorkerHealthStatusHealthy // Fallback for any future status values
		}

		worker := api.Worker{
			Id:              w.WorkerId,
			Labels:          w.Labels,
			TotalPollers:    int(w.TotalPollers),
			BusyPollers:     int(w.BusyPollers),
			RunningTasks:    transformRunningTasks(w.RunningTasks),
			LastHeartbeatAt: time.Unix(w.LastHeartbeatAt, 0).Format(time.RFC3339),
			HealthStatus:    healthStatus,
		}
		workers = append(workers, worker)
	}

	return api.GetWorkers200JSONResponse{
		Workers: workers,
		Errors:  errors,
	}, nil
}

// transformRunningTasks converts proto running tasks to API running tasks
func transformRunningTasks(tasks []*coordinatorv1.RunningTask) []api.RunningTask {
	result := make([]api.RunningTask, len(tasks))
	for i, task := range tasks {
		result[i] = api.RunningTask{
			DagRunId:         task.DagRunId,
			DagName:          task.DagName,
			StartedAt:        time.Unix(task.StartedAt, 0).Format(time.RFC3339),
			RootDagRunName:   &task.RootDagRunName,
			RootDagRunId:     &task.RootDagRunId,
			ParentDagRunName: &task.ParentDagRunName,
			ParentDagRunId:   &task.ParentDagRunId,
		}
	}
	return result
}
