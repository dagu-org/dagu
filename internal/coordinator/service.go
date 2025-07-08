package coordinator

import (
	"context"
	"net"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Service struct {
	server       *grpc.Server
	handler      *Handler
	grpcListener net.Listener
	healthServer *health.Server
}

func NewService(
	server *grpc.Server,
	handler *Handler,
	grpcListener net.Listener,
	healthServer *health.Server,
) *Service {
	return &Service{
		server:       server,
		handler:      handler,
		grpcListener: grpcListener,
		healthServer: healthServer,
	}
}

func (srv *Service) Start(ctx context.Context) error {
	coordinatorv1.RegisterCoordinatorServiceServer(srv.server, srv.handler)

	// Set the serving status for the coordinator service
	srv.healthServer.SetServingStatus(coordinatorv1.CoordinatorService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	// Also set the overall server status
	srv.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	go func() {
		logger.Info(ctx, "Starting to serve on coordinator service")
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

	t := time.AfterFunc(2*time.Second, func() {
		logger.Info(ctx, "ShutdownHandler: Drain time expired, stopping all traffic")
		srv.server.Stop()
	})

	srv.server.GracefulStop()
	t.Stop()

	return nil
}
