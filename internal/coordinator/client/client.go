package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Client defines the interface for coordinator operations
type Client interface {
	// Dispatch sends a task to the coordinator
	Dispatch(ctx context.Context, task *coordinatorv1.Task) error

	// GetTaskStatus queries the status of a dispatched task
	// Note: This is a placeholder for future implementation when the coordinator
	// supports status queries
	GetTaskStatus(ctx context.Context, dagRunID string) (*TaskStatus, error)

	// Close closes the client connection
	Close() error
}

// TaskStatus represents the status of a dispatched task
type TaskStatus struct {
	State       string // "pending", "running", "completed", "failed"
	Error       string // Error message if failed
	StartedAt   time.Time
	CompletedAt time.Time
}

// coordinatorClient is the concrete implementation
type coordinatorClient struct {
	conn         *grpc.ClientConn
	client       coordinatorv1.CoordinatorServiceClient
	healthClient grpc_health_v1.HealthClient
	config       *Config
}

// Errors
var (
	ErrInvalidHost      = fmt.Errorf("invalid host")
	ErrInvalidPort      = fmt.Errorf("invalid port")
	ErrMissingTLSConfig = fmt.Errorf("TLS enabled but no certificates provided")
	ErrNotConnected     = fmt.Errorf("not connected to coordinator")
	ErrTaskNotFound     = fmt.Errorf("task not found")
)

// newClient creates a new coordinator client with the given configuration
func newClient(ctx context.Context, config *Config) (*coordinatorClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Get dial options based on TLS configuration
	dialOpts, err := getDialOptions(config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure gRPC connection: %w", err)
	}

	// Create gRPC connection using the newer NewClient API
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator client for %s: %w", addr, err)
	}

	client := &coordinatorClient{
		conn:         conn,
		client:       coordinatorv1.NewCoordinatorServiceClient(conn),
		healthClient: grpc_health_v1.NewHealthClient(conn),
		config:       config,
	}

	// Wait for coordinator to be healthy
	if err := client.waitForHealthy(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("coordinator not healthy: %w", err)
	}

	logger.Info(ctx, "Coordinator client connected",
		"host", config.Host,
		"port", config.Port,
		"insecure", config.Insecure,
	)

	return client, nil
}

// Dispatch sends a task to the coordinator
func (c *coordinatorClient) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {
	if c.client == nil {
		return ErrNotConnected
	}

	// Create request
	req := &coordinatorv1.DispatchRequest{
		Task: task,
	}

	// Apply request timeout
	dispatchCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	// Dispatch with retry
	err := c.dispatchWithRetry(dispatchCtx, req)
	if err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	logger.Info(ctx, "Task dispatched successfully",
		"dag_run_id", task.DagRunId,
		"target", task.Target,
		"worker_selector", task.WorkerSelector,
	)

	return nil
}

// dispatchWithRetry attempts to dispatch a task with exponential backoff retry
func (c *coordinatorClient) dispatchWithRetry(ctx context.Context, req *coordinatorv1.DispatchRequest) error {
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

// GetTaskStatus queries the status of a dispatched task
// Note: This is a placeholder implementation. The coordinator service
// doesn't currently support status queries.
func (c *coordinatorClient) GetTaskStatus(ctx context.Context, dagRunID string) (*TaskStatus, error) {
	// TODO: Implement when coordinator supports status queries
	return nil, fmt.Errorf("task status query not implemented")
}

// Close closes the client connection
func (c *coordinatorClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// waitForHealthy waits for the coordinator to become healthy
func (c *coordinatorClient) waitForHealthy(ctx context.Context) error {
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
			return err
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("coordinator not healthy: %s", resp.Status)
		}

		return nil
	}, policy, nil)
}

// getDialOptions returns the appropriate gRPC dial options based on TLS configuration
func getDialOptions(config *Config) ([]grpc.DialOption, error) {
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
