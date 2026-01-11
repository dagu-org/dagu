package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Service struct {
	server         *grpc.Server
	handler        *Handler
	grpcListener   net.Listener
	healthServer   *health.Server
	registry       execution.ServiceRegistry
	cfg            *config.Config
	instanceID     string
	hostPort       string
	configuredHost string

	// For graceful shutdown
	stopCancel context.CancelFunc // Cancels the service's internal context
}

func NewService(
	server *grpc.Server,
	handler *Handler,
	grpcListener net.Listener,
	healthServer *health.Server,
	registry execution.ServiceRegistry,
	cfg *config.Config,
	instanceID string,
	configuredHost string,
) *Service {
	return &Service{
		server:         server,
		handler:        handler,
		grpcListener:   grpcListener,
		healthServer:   healthServer,
		registry:       registry,
		cfg:            cfg,
		instanceID:     instanceID,
		hostPort:       grpcListener.Addr().String(),
		configuredHost: configuredHost,
	}
}

func (srv *Service) Start(ctx context.Context) error {
	coordinatorv1.RegisterCoordinatorServiceServer(srv.server, srv.handler)

	// Set the serving status for the coordinator service
	srv.healthServer.SetServingStatus(coordinatorv1.CoordinatorService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	// Also set the overall server status
	srv.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Create an internal context that can be cancelled by Stop()
	internalCtx, cancel := context.WithCancel(ctx)
	srv.stopCancel = cancel

	// Start the zombie detector to clean up runs from crashed workers
	// Use configured interval or default to 45 seconds
	zombieInterval := 45 * time.Second
	if srv.cfg != nil && srv.cfg.Scheduler.ZombieDetectionInterval > 0 {
		zombieInterval = srv.cfg.Scheduler.ZombieDetectionInterval
	}
	srv.handler.StartZombieDetector(internalCtx, zombieInterval)
	logger.Info(ctx, "Started zombie detector", tag.Interval(zombieInterval))

	// Register with service registry if monitor is available
	if srv.registry != nil {
		// Parse port from listener address
		_, portStr, err := net.SplitHostPort(srv.hostPort)
		if err != nil {
			return fmt.Errorf("failed to parse host:port: %w", err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("failed to parse port number: %w", err)
		}

		hostInfo := execution.HostInfo{
			ID:     srv.instanceID,
			Host:   srv.configuredHost,
			Port:   port,
			Status: execution.ServiceStatusActive, // Coordinator is active when serving
		}
		if err := srv.registry.Register(ctx, execution.ServiceNameCoordinator, hostInfo); err != nil {
			return fmt.Errorf("failed to register with service registry: %w", err)
		}
		logger.Info(ctx, "Registered with service registry",
			tag.ServiceID(srv.instanceID),
			slog.String("configured-host", srv.configuredHost),
			tag.Port(port),
			tag.Addr(srv.hostPort))
	}

	go func() {
		logger.Info(ctx, "Starting to serve on coordinator service", tag.Addr(srv.hostPort))
		if err := srv.server.Serve(srv.grpcListener); err != nil {
			logger.Fatal(ctx, "Failed to serve on coordinator service listener")
		}
	}()

	return nil
}

func (srv *Service) Stop(ctx context.Context) error {
	// Set NOT_SERVING status when shutting down
	srv.healthServer.SetServingStatus(coordinatorv1.CoordinatorService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	srv.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Unregister from service registry if monitor is available
	if srv.registry != nil {
		srv.registry.Unregister(ctx)
		logger.Info(ctx, "Unregistered from service registry", tag.ServiceID(srv.instanceID))
	}

	t := time.AfterFunc(2*time.Second, func() {
		logger.Info(ctx, "ShutdownHandler: Drain time expired, stopping all traffic")
		srv.server.Stop()
	})

	srv.server.GracefulStop()
	t.Stop()

	// Cancel the internal context to signal zombie detector to stop
	if srv.stopCancel != nil {
		srv.stopCancel()
	}

	// Wait for zombie detector to finish before closing handler
	srv.handler.WaitZombieDetector()

	// Close handler resources (open attempts, etc.)
	srv.handler.Close(ctx)

	return nil
}
