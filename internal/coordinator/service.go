package coordinator

import (
	"context"
	"net"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
)

type Service struct {
	server       *grpc.Server
	handler      *Handler
	grpcListener net.Listener
}

func NewService(
	server *grpc.Server,
	handler *Handler,
	grpcListener net.Listener,
) *Service {
	return &Service{
		server:       server,
		handler:      handler,
		grpcListener: grpcListener,
	}
}

func (srv *Service) Start(ctx context.Context) error {
	coordinatorv1.RegisterCoordinatorServiceServer(srv.server, srv.handler)

	go func() {
		logger.Info(ctx, "Starting to serve on coordinator service")
		if err := srv.server.Serve(srv.grpcListener); err != nil {
			logger.Fatal(ctx, "Failed to serve on coordinator service listener")
		}
	}()

	return nil
}

func (srv *Service) Stop(ctx context.Context) error {
	t := time.AfterFunc(2*time.Second, func() {
		logger.Info(ctx, "ShutdownHandler: Drain time expired, stopping all traffic")
		srv.server.Stop()
	})

	srv.server.GracefulStop()
	t.Stop()

	return nil
}
