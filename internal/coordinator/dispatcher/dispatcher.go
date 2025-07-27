package dispatcher

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Dispatcher defines the interface for coordinator operations
type Dispatcher interface {
	// Dispatch sends a task to the coordinator
	Dispatch(ctx context.Context, task *coordinatorv1.Task) error

	// Dispose cleans up any resources used by the dispatcher
	Dispose(ctx context.Context) error
}

// client holds the gRPC connection and clients for the coordinator service.
// it should be closed and removed when no longer needed or when the coordinator
// is unhealthy.
type client struct {
	conn         *grpc.ClientConn
	client       coordinatorv1.CoordinatorServiceClient
	healthClient grpc_health_v1.HealthClient
}

var _ Dispatcher = (*dispatcher)(nil)

// dispatcher is the concrete implementation
type dispatcher struct {
	config    *Config
	discovery models.ServiceMonitor
}

// Errors
var (
	ErrMissingTLSConfig = fmt.Errorf("TLS enabled but no certificates provided")
)

// newClient creates a new coordinator client with the given configuration
func NewDispatcher(ctx context.Context, monitor models.ServiceMonitor, config *Config) (Dispatcher, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &dispatcher{
		config:    config,
		discovery: monitor,
	}, nil
}

// Dispatch sends a task to the coordinator
func (c *dispatcher) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {

	// TODO: get members from discovery service , shuffle them and try each one
	// create gRPC connection and cache the client for same member id

	return nil
}

// dispatchWithRetry attempts to dispatch a task with exponential backoff retry
func (c *dispatcher) dispatchWithRetry(ctx context.Context, req *coordinatorv1.DispatchRequest) error {
	if c.config.MaxRetries == 0 {
		// No retries, just try once
		_, err := c.client.Dispatch(ctx, req)
		return err
	}

	// Set up retry policy
	basePolicy := backoff.NewExponentialBackoffPolicy(c.config.RetryInterval)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = 30 * time.Second
	basePolicy.MaxRetries = c.config.MaxRetries

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	return backoff.Retry(ctx, func(ctx context.Context) error {
		_, err := c.client.Dispatch(ctx, req)
		return err
	}, policy, nil)
}

// waitForHealthy waits for the coordinator to become healthy
func (c *client) waitForHealthy(ctx context.Context) error {
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = 30 * time.Second
	basePolicy.MaxRetries = 10

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	return backoff.Retry(ctx, func(ctx context.Context) error {
		req := &grpc_health_v1.HealthCheckRequest{
			Service: "", // Check overall server health
		}

		resp, err := c.healthClient.Check(ctx, req)
		if err != nil {
			logger.Error(ctx, "Health check failed", "err", err)
			return err
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("coordinator not healthy: %s", resp.Status)
		}

		return nil
	}, policy, nil)
}

// getDialOptions returns the appropriate gRPC dial options based on TLS configuration
func getDialOptions(address string, config *Config) ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{}

	if config.Insecure {
		// Use insecure connection (h2c)
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return opts, nil
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Set InsecureSkipVerify if requested
	if config.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	// Load client certificates if provided
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if config.CAFile != "" {
		caData, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		certPool, err := x509.SystemCertPool()
		if err != nil {
			// Fall back to empty pool
			certPool = x509.NewCertPool()
		}

		if !certPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = certPool
	}

	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	return opts, nil
}
