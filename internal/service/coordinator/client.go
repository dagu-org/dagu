package coordinator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Client abstracts handling communication with the coordinator service using
// service registry and gRPC.
type Client interface {
	execution.Dispatcher

	// Dispatch sends a task to the coordinator
	Dispatch(ctx context.Context, task *coordinatorv1.Task) error

	// Poll retrieves a task from the coordinator.
	Poll(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error)

	// GetWorkers retrieves the list of workers from the coordinator
	GetWorkers(ctx context.Context) ([]*coordinatorv1.WorkerInfo, error)

	// Heartbeat sends a heartbeat to the coordinator and returns the response
	// which may include cancellation directives
	Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error)

	// ReportStatus sends a status update to the coordinator (for shared-nothing workers)
	ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error)

	// StreamLogs returns a log streaming client for sending logs to the coordinator
	StreamLogs(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error)

	// RequestCancel requests cancellation of a DAG run through the coordinator.
	// Used in shared-nothing mode for sub-DAG cancellation.
	RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *execution.DAGRunRef) error

	// GetDAGRunStatus is inherited from execution.Dispatcher

	// Metrics returns the metrics for the coordinator client
	Metrics() Metrics
}

// Metrics defines the metrics for the coordinator client
type Metrics struct {
	FailCount        int   // Total number of failures
	IsConnected      bool  // Whether the client is currently connected
	ConsecutiveFails int   // Number of consecutive failures
	LastError        error // Last error encountered
}

var (
	_ Client               = (*clientImpl)(nil)
	_ execution.Dispatcher = (*clientImpl)(nil)
)

// clientImpl is the concrete implementation
type clientImpl struct {
	config   *Config
	registry execution.ServiceRegistry

	clientsMu sync.RWMutex
	clients   map[string]*client // Cache of gRPC clients by coordinator ID

	stateMu sync.RWMutex // Mutex for state access
	state   *Metrics     // Connection state tracking
}

// client holds the gRPC connection and clients for the coordinator service.
// it should be closed and removed when no longer needed or when the coordinator
// is unhealthy.
type client struct {
	conn         *grpc.ClientConn
	client       coordinatorv1.CoordinatorServiceClient
	healthClient grpc_health_v1.HealthClient
}

// Errors
var (
	ErrMissingTLSConfig = fmt.Errorf("TLS enabled but no certificates provided")
)

// New creates a new coordinator client with the given configuration
func New(registry execution.ServiceRegistry, config *Config) Client {
	return &clientImpl{
		config:   config,
		registry: registry,
		clients:  make(map[string]*client),
		state: &Metrics{
			IsConnected: true, // Assume connected initially
		},
	}
}

// Dispatch sends a task to the coordinator
func (cli *clientImpl) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {
	// Set up retry policy
	basePolicy := backoff.NewExponentialBackoffPolicy(cli.config.RetryInterval)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = 30 * time.Second
	basePolicy.MaxRetries = cli.config.MaxRetries

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	return backoff.Retry(ctx, func(ctx context.Context) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		members, err := cli.getCoordinatorMembers(ctx)
		if err != nil {
			return err
		}

		return cli.attemptCall(ctx, members, func(ctx context.Context, member execution.HostInfo, client *client) error {
			// Create request
			req := &coordinatorv1.DispatchRequest{Task: task}

			// Apply request timeout
			dispatchCtx, cancel := context.WithTimeout(ctx, cli.config.RequestTimeout)
			defer cancel()

			// Try to dispatch
			if _, err := client.client.Dispatch(dispatchCtx, req); err != nil {
				logger.Warn(ctx, "Failed to dispatch task to coordinator",
					tag.RunID(task.DagRunId),
					tag.Target(task.Target),
					slog.Any("worker-selector", task.WorkerSelector),
					slog.String("coordinator-id", member.ID),
				)

				return fmt.Errorf("failed to dispatch task to coordinator %s: %w", member.ID, err)
			}

			logger.Info(ctx, "Task dispatched successfully",
				tag.RunID(task.DagRunId),
				tag.Target(task.Target),
				slog.Any("worker-selector", task.WorkerSelector),
				slog.String("coordinator-id", member.ID),
			)

			return nil
		})
	}, policy, nil)
}

// Poll implements Client.
func (cli *clientImpl) Poll(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
	var task *coordinatorv1.Task
	err := backoff.Retry(ctx, func(ctx context.Context) error {
		members, err := cli.getCoordinatorMembers(ctx)
		if err != nil {
			return err
		}

		return cli.attemptCall(ctx, members, func(ctx context.Context, member execution.HostInfo, client *client) error {
			resp, err := client.client.Poll(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to poll task from coordinator %s: %w", member.ID, err)
			}

			if resp.Task != nil {
				task = resp.Task
				logger.Info(ctx, "Task polled successfully",
					tag.RunID(task.DagRunId),
					tag.Target(task.Target),
					slog.Any("worker-selector", task.WorkerSelector),
					slog.String("coordinator-id", member.ID),
				)
			}

			return nil
		})

	}, policy, nil)

	return task, err
}

// Metrics implements Client.
func (cli *clientImpl) Metrics() Metrics {
	cli.stateMu.RLock()
	defer cli.stateMu.RUnlock()

	return *cli.state
}

func (cli *clientImpl) attemptCall(ctx context.Context, members []execution.HostInfo, callback func(ctx context.Context, member execution.HostInfo, client *client) error) error {
	// Shuffle members to distribute load evenly
	rand.Shuffle(len(members), func(i, j int) {
		members[i], members[j] = members[j], members[i]
	})

	// Try each coordinator in order (round-robin style)
	var lastErr error
	for _, member := range members {
		// Get or create client for this coordinator
		client, err := cli.getOrCreateClient(member)
		if err != nil {
			logger.Warn(ctx, "Failed to connect to coordinator",
				slog.String("coordinator-id", member.ID),
				tag.Host(member.Host),
				tag.Port(member.Port),
				tag.Error(err))
			cli.removeClient(member.ID) // Remove failed client
			cli.recordFailure(err)
			lastErr = err
			continue
		}

		// Check if the coordinator is healthy
		if err := cli.isHealthy(ctx, member); err != nil {
			logger.Warn(ctx, "Failed to check coordinator health",
				slog.String("coordinator-id", member.ID),
				tag.Host(member.Host),
				tag.Port(member.Port),
				tag.Error(err))
			cli.recordFailure(err)
			lastErr = err
			continue
		}

		// Create request
		if err := callback(ctx, member, client); err != nil {
			logger.Debug(ctx, "Failed to dispatch to coordinator",
				slog.String("coordinator-id", member.ID),
				tag.Host(member.Host),
				tag.Port(member.Port),
				tag.Error(err))
			lastErr = err
			cli.recordFailure(err)
		} else {
			// Success - record and return immediately
			cli.recordSuccess(ctx)
			return nil
		}
	}

	return lastErr
}

func (cli *clientImpl) isHealthy(ctx context.Context, member execution.HostInfo) error {
	// Get or create client for this coordinator
	client, err := cli.getOrCreateClient(member)
	if err != nil {
		return fmt.Errorf("failed to get coordinator client: %w", err)
	}

	// Check health
	req := &grpc_health_v1.HealthCheckRequest{
		Service: "", // Check overall server health
	}

	resp, err := client.healthClient.Check(ctx, req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("coordinator not healthy: %s", resp.Status)
	}

	return nil
}

// getOrCreateClient gets an existing client or creates a new one for the given member
func (cli *clientImpl) getOrCreateClient(member execution.HostInfo) (*client, error) {
	// Try to get existing client with read lock
	cli.clientsMu.RLock()
	if c, exists := cli.clients[member.ID]; exists {
		cli.clientsMu.RUnlock()
		return c, nil
	}
	cli.clientsMu.RUnlock()

	// Need to create new client, acquire write lock
	cli.clientsMu.Lock()
	defer cli.clientsMu.Unlock()

	// Double-check after acquiring write lock
	if c, exists := cli.clients[member.ID]; exists {
		return c, nil
	}

	// Create new client
	c, err := cli.createClient(member)
	if err != nil {
		return nil, err
	}

	// Cache it
	cli.clients[member.ID] = c
	return c, nil
}

// createClient creates a new gRPC client for the given coordinator
func (cli *clientImpl) createClient(member execution.HostInfo) (*client, error) {
	// Get dial options based on TLS configuration
	dialOpts, err := getDialOptions(cli.config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure gRPC connection: %w", err)
	}

	// Construct address from host and port
	address := fmt.Sprintf("%s:%d", member.Host, member.Port)

	// Create gRPC connection
	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator client for %s: %w", address, err)
	}

	return &client{
		conn:         conn,
		client:       coordinatorv1.NewCoordinatorServiceClient(conn),
		healthClient: grpc_health_v1.NewHealthClient(conn),
	}, nil
}

// removeClient removes a client from the cache
func (cli *clientImpl) removeClient(coordinatorID string) {
	cli.clientsMu.Lock()
	defer cli.clientsMu.Unlock()

	if c, exists := cli.clients[coordinatorID]; exists {
		_ = c.conn.Close()
		delete(cli.clients, coordinatorID)
	}
}

// Cleanup cleans up all connections
func (cli *clientImpl) Cleanup(ctx context.Context) error {
	cli.clientsMu.Lock()
	defer cli.clientsMu.Unlock()

	for id, c := range cli.clients {
		if err := c.conn.Close(); err != nil {
			logger.Error(ctx, "Failed to close connection", slog.String("coordinator-id", id), tag.Error(err))
		}
	}

	// Clear the map
	cli.clients = make(map[string]*client)
	return nil
}

// recordFailure updates the state for a failed coordinator connection/operation
func (cli *clientImpl) recordFailure(err error) {
	cli.stateMu.Lock()
	defer cli.stateMu.Unlock()

	cli.state.IsConnected = false
	cli.state.ConsecutiveFails++
	cli.state.FailCount++
	cli.state.LastError = err
}

// recordSuccess updates the state for a successful coordinator operation
func (cli *clientImpl) recordSuccess(ctx context.Context) {
	cli.stateMu.Lock()
	defer cli.stateMu.Unlock()

	// Log recovery if this was a disconnection
	if !cli.state.IsConnected && cli.state.ConsecutiveFails > 0 {
		logger.Info(ctx, "CoordinatorCli connection recovered",
			slog.Int("previous-consecutive-failures", cli.state.ConsecutiveFails))
	}

	// Reset consecutive failures on success
	cli.state.IsConnected = true
	cli.state.ConsecutiveFails = 0
	cli.state.LastError = nil
}

// getCoordinatorMembers discovers available coordinators from the service registry.
// Returns an error if discovery fails or no coordinators are available.
func (cli *clientImpl) getCoordinatorMembers(ctx context.Context) ([]execution.HostInfo, error) {
	members, err := cli.registry.GetServiceMembers(ctx, execution.ServiceNameCoordinator)
	if err != nil {
		return nil, fmt.Errorf("failed to discover coordinators: %w", err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("no coordinators available")
	}
	return members, nil
}

// GetWorkers retrieves the list of workers from all coordinators
func (cli *clientImpl) GetWorkers(ctx context.Context) ([]*coordinatorv1.WorkerInfo, error) {
	// Get all available coordinators from discovery
	members, err := cli.registry.GetServiceMembers(ctx, execution.ServiceNameCoordinator)
	if err != nil {
		return nil, fmt.Errorf("failed to discover coordinators: %w", err)
	}

	// Collect workers from all coordinators
	var allWorkers []*coordinatorv1.WorkerInfo
	var lastErr error

	for _, member := range members {
		// Get or create client for this member
		c, err := cli.getOrCreateClient(member)
		if err != nil {
			logger.Warn(ctx, "Failed to connect to coordinator",
				tag.ID(member.ID),
				tag.Host(member.Host),
				tag.Port(member.Port),
				tag.Error(err))
			lastErr = err
			continue
		}

		// Try to get workers from this coordinator
		resp, err := c.client.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		if err != nil {
			logger.Warn(ctx, "Failed to get workers from coordinator",
				tag.ID(member.ID),
				tag.Host(member.Host),
				tag.Port(member.Port),
				tag.Error(err))
			lastErr = err

			// If this is a connection error, remove the client from cache
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
				cli.removeClient(member.ID)
			}
			continue
		}

		// Append workers from this coordinator
		if resp != nil && resp.Workers != nil {
			allWorkers = append(allWorkers, resp.Workers...)
		}
	}

	// If we got some workers, return them even if some coordinators failed
	if len(allWorkers) > 0 {
		return allWorkers, nil
	}

	// All attempts failed and no workers found
	if lastErr != nil {
		return nil, fmt.Errorf("failed to get workers from any coordinator: %w", lastErr)
	}

	return allWorkers, nil
}

// Heartbeat sends a heartbeat to coordinators and returns the response
func (cli *clientImpl) Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	members, err := cli.getCoordinatorMembers(ctx)
	if err != nil {
		return nil, err
	}

	var resp *coordinatorv1.HeartbeatResponse
	err = cli.attemptCall(ctx, members, func(ctx context.Context, _ execution.HostInfo, client *client) error {
		var callErr error
		resp, callErr = client.client.Heartbeat(ctx, req)
		if callErr != nil {
			return fmt.Errorf("heartbeat failed: %w", callErr)
		}
		return nil
	})
	return resp, err
}

// ReportStatus sends a status update to the coordinator
func (cli *clientImpl) ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
	members, err := cli.getCoordinatorMembers(ctx)
	if err != nil {
		return nil, err
	}

	var resp *coordinatorv1.ReportStatusResponse
	err = cli.attemptCall(ctx, members, func(ctx context.Context, _ execution.HostInfo, client *client) error {
		var callErr error
		resp, callErr = client.client.ReportStatus(ctx, req)
		if callErr != nil {
			return fmt.Errorf("report status failed: %w", callErr)
		}
		return nil
	})
	return resp, err
}

// StreamLogs returns a log streaming client for sending logs to the coordinator.
// It performs health checks and tries multiple coordinators for failover.
func (cli *clientImpl) StreamLogs(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
	members, err := cli.getCoordinatorMembers(ctx)
	if err != nil {
		return nil, err
	}

	// Shuffle members to distribute load evenly
	rand.Shuffle(len(members), func(i, j int) {
		members[i], members[j] = members[j], members[i]
	})

	// Try each coordinator until one works
	var lastErr error
	for _, member := range members {
		client, err := cli.getOrCreateClient(member)
		if err != nil {
			cli.recordFailure(err)
			lastErr = err
			continue
		}

		// Check if the coordinator is healthy before streaming
		if err := cli.isHealthy(ctx, member); err != nil {
			cli.recordFailure(err)
			lastErr = err
			continue
		}

		stream, err := client.client.StreamLogs(ctx)
		if err != nil {
			cli.recordFailure(err)
			lastErr = err
			continue
		}

		cli.recordSuccess(ctx)
		return stream, nil
	}

	return nil, fmt.Errorf("failed to create log stream: %w", lastErr)
}

// GetDAGRunStatus retrieves the status of a DAG run from the coordinator
func (cli *clientImpl) GetDAGRunStatus(ctx context.Context, dagName, dagRunID string, rootRef *execution.DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error) {
	members, err := cli.getCoordinatorMembers(ctx)
	if err != nil {
		return nil, err
	}

	req := &coordinatorv1.GetDAGRunStatusRequest{
		DagName:  dagName,
		DagRunId: dagRunID,
	}

	// Include root reference for sub-DAG queries
	if rootRef != nil {
		req.RootDagRunName = rootRef.Name
		req.RootDagRunId = rootRef.ID
	}

	var resp *coordinatorv1.GetDAGRunStatusResponse
	err = cli.attemptCall(ctx, members, func(ctx context.Context, _ execution.HostInfo, client *client) error {
		var callErr error
		resp, callErr = client.client.GetDAGRunStatus(ctx, req)
		if callErr != nil {
			return fmt.Errorf("get DAG run status failed: %w", callErr)
		}
		return nil
	})
	return resp, err
}

// RequestCancel requests cancellation of a DAG run through the coordinator
func (cli *clientImpl) RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *execution.DAGRunRef) error {
	members, err := cli.getCoordinatorMembers(ctx)
	if err != nil {
		return err
	}

	req := &coordinatorv1.RequestCancelRequest{
		DagName:  dagName,
		DagRunId: dagRunID,
	}

	// Include root reference for sub-DAG cancellation
	if rootRef != nil {
		req.RootDagRunName = rootRef.Name
		req.RootDagRunId = rootRef.ID
	}

	return cli.attemptCall(ctx, members, func(ctx context.Context, _ execution.HostInfo, client *client) error {
		resp, callErr := client.client.RequestCancel(ctx, req)
		if callErr != nil {
			return fmt.Errorf("request cancel failed: %w", callErr)
		}
		if !resp.Accepted {
			return fmt.Errorf("cancellation not accepted: %s", resp.Error)
		}
		return nil
	})
}

// getDialOptions returns the appropriate gRPC dial options based on TLS configuration
func getDialOptions(config *Config) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

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
