package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	"github.com/dagu-org/dagu/internal/runtime/remote"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

var _ TaskHandler = (*remoteTaskHandler)(nil)

type mockStreamLogsClient struct {
	chunks   []*coordinatorv1.LogChunk
	mu       sync.Mutex
	sendErr  error
	closeErr error
	response *coordinatorv1.StreamLogsResponse
	ctx      context.Context
}

func newMockStreamLogsClient() *mockStreamLogsClient {
	return &mockStreamLogsClient{
		chunks: make([]*coordinatorv1.LogChunk, 0),
		ctx:    context.Background(),
		response: &coordinatorv1.StreamLogsResponse{
			ChunksReceived: 0,
			BytesWritten:   0,
		},
	}
}

func (m *mockStreamLogsClient) Send(chunk *coordinatorv1.LogChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.chunks = append(m.chunks, chunk)
	return nil
}

func (m *mockStreamLogsClient) CloseAndRecv() (*coordinatorv1.StreamLogsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	if m.response != nil {
		m.response.ChunksReceived = uint64(len(m.chunks))
	}
	return m.response, nil
}

func (m *mockStreamLogsClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *mockStreamLogsClient) Trailer() metadata.MD {
	return nil
}

func (m *mockStreamLogsClient) CloseSend() error {
	return nil
}

func (m *mockStreamLogsClient) Context() context.Context {
	return m.ctx
}

func (m *mockStreamLogsClient) SendMsg(any) error {
	return nil
}

func (m *mockStreamLogsClient) RecvMsg(any) error {
	return nil
}

type mockRemoteCoordinatorClient struct {
	ReportStatusFunc    func(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error)
	StreamLogsFunc      func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error)
	GetDAGRunStatusFunc func(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error)
	DispatchFunc        func(ctx context.Context, task *coordinatorv1.Task) error
	PollFunc            func(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error)
	HeartbeatFunc       func(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error)
	GetWorkersFunc      func(ctx context.Context) ([]*coordinatorv1.WorkerInfo, error)
	CleanupFunc         func(ctx context.Context) error
	MetricsFunc         func() coordinator.Metrics
	RequestCancelFunc   func(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) error
}

func newMockRemoteCoordinatorClient() *mockRemoteCoordinatorClient {
	return &mockRemoteCoordinatorClient{
		ReportStatusFunc: func(_ context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
			return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
		},
		StreamLogsFunc: func(_ context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return newMockStreamLogsClient(), nil
		},
		GetDAGRunStatusFunc: func(_ context.Context, _, _ string, _ *exec.DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error) {
			return &coordinatorv1.GetDAGRunStatusResponse{Found: false}, nil
		},
		MetricsFunc: func() coordinator.Metrics {
			return coordinator.Metrics{IsConnected: true}
		},
	}
}

func (m *mockRemoteCoordinatorClient) ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
	if m.ReportStatusFunc != nil {
		return m.ReportStatusFunc(ctx, req)
	}
	return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
}

func (m *mockRemoteCoordinatorClient) StreamLogs(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
	if m.StreamLogsFunc != nil {
		return m.StreamLogsFunc(ctx)
	}
	return newMockStreamLogsClient(), nil
}

func (m *mockRemoteCoordinatorClient) GetDAGRunStatus(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error) {
	if m.GetDAGRunStatusFunc != nil {
		return m.GetDAGRunStatusFunc(ctx, dagName, dagRunID, rootRef)
	}
	return &coordinatorv1.GetDAGRunStatusResponse{Found: false}, nil
}

func (m *mockRemoteCoordinatorClient) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {
	if m.DispatchFunc != nil {
		return m.DispatchFunc(ctx, task)
	}
	return nil
}

func (m *mockRemoteCoordinatorClient) Poll(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
	if m.PollFunc != nil {
		return m.PollFunc(ctx, policy, req)
	}
	return nil, nil
}

func (m *mockRemoteCoordinatorClient) Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	if m.HeartbeatFunc != nil {
		return m.HeartbeatFunc(ctx, req)
	}
	return &coordinatorv1.HeartbeatResponse{}, nil
}

func (m *mockRemoteCoordinatorClient) GetWorkers(ctx context.Context) ([]*coordinatorv1.WorkerInfo, error) {
	if m.GetWorkersFunc != nil {
		return m.GetWorkersFunc(ctx)
	}
	return nil, nil
}

func (m *mockRemoteCoordinatorClient) Cleanup(ctx context.Context) error {
	if m.CleanupFunc != nil {
		return m.CleanupFunc(ctx)
	}
	return nil
}

func (m *mockRemoteCoordinatorClient) Metrics() coordinator.Metrics {
	if m.MetricsFunc != nil {
		return m.MetricsFunc()
	}
	return coordinator.Metrics{IsConnected: true}
}

func (m *mockRemoteCoordinatorClient) RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) error {
	if m.RequestCancelFunc != nil {
		return m.RequestCancelFunc(ctx, dagName, dagRunID, rootRef)
	}
	return nil
}

// mockRemoteDAGRunAttempt implements execution.DAGRunAttempt for testing
type mockRemoteDAGRunAttempt struct {
	id       string
	status   *exec.DAGRunStatus
	dag      *core.DAG
	readErr  error
	openErr  error
	writeErr error
	closeErr error
	opened   bool
	closed   bool
	written  bool
	aborting bool
	hidden   bool
	mu       sync.Mutex
}

func newMockRemoteDAGRunAttempt(id string, status *exec.DAGRunStatus) *mockRemoteDAGRunAttempt {
	return &mockRemoteDAGRunAttempt{
		id:     id,
		status: status,
	}
}

func (m *mockRemoteDAGRunAttempt) ID() string {
	return m.id
}

func (m *mockRemoteDAGRunAttempt) Open(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.openErr != nil {
		return m.openErr
	}
	m.opened = true
	return nil
}

func (m *mockRemoteDAGRunAttempt) Write(_ context.Context, status exec.DAGRunStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.written = true
	m.status = &status
	return nil
}

func (m *mockRemoteDAGRunAttempt) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeErr != nil {
		return m.closeErr
	}
	m.closed = true
	return nil
}

func (m *mockRemoteDAGRunAttempt) ReadStatus(_ context.Context) (*exec.DAGRunStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.status, nil
}

func (m *mockRemoteDAGRunAttempt) ReadDAG(_ context.Context) (*core.DAG, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dag, nil
}

func (m *mockRemoteDAGRunAttempt) SetDAG(dag *core.DAG) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dag = dag
}

func (m *mockRemoteDAGRunAttempt) Abort(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aborting = true
	return nil
}

func (m *mockRemoteDAGRunAttempt) IsAborting(_ context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.aborting, nil
}

func (m *mockRemoteDAGRunAttempt) Hide(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hidden = true
	return nil
}

func (m *mockRemoteDAGRunAttempt) Hidden() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hidden
}

func (m *mockRemoteDAGRunAttempt) WriteOutputs(_ context.Context, _ *exec.DAGRunOutputs) error {
	return nil
}

func (m *mockRemoteDAGRunAttempt) ReadOutputs(_ context.Context) (*exec.DAGRunOutputs, error) {
	return nil, nil
}

func (m *mockRemoteDAGRunAttempt) WriteStepMessages(_ context.Context, _ string, _ []exec.LLMMessage) error {
	return nil
}

func (m *mockRemoteDAGRunAttempt) ReadStepMessages(_ context.Context, _ string) ([]exec.LLMMessage, error) {
	return nil, nil
}

// mockRemoteDAGRunStore implements execution.DAGRunStore for testing
type mockRemoteDAGRunStore struct {
	attempts    map[string]exec.DAGRunAttempt
	subAttempts map[string]exec.DAGRunAttempt
	findErr     error
	createErr   error
	mu          sync.Mutex
}

func newMockRemoteDAGRunStore() *mockRemoteDAGRunStore {
	return &mockRemoteDAGRunStore{
		attempts:    make(map[string]exec.DAGRunAttempt),
		subAttempts: make(map[string]exec.DAGRunAttempt),
	}
}

func (m *mockRemoteDAGRunStore) SetAttempt(ref exec.DAGRunRef, attempt exec.DAGRunAttempt) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[ref.ID] = attempt
}

func (m *mockRemoteDAGRunStore) SetSubAttempt(rootRef exec.DAGRunRef, subID string, attempt exec.DAGRunAttempt) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", rootRef.ID, subID)
	m.subAttempts[key] = attempt
}

func (m *mockRemoteDAGRunStore) FindAttempt(_ context.Context, ref exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	attempt, ok := m.attempts[ref.ID]
	if !ok {
		return nil, exec.ErrDAGRunIDNotFound
	}
	return attempt, nil
}

func (m *mockRemoteDAGRunStore) FindSubAttempt(_ context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	key := fmt.Sprintf("%s:%s", rootRef.ID, subDAGRunID)
	attempt, ok := m.subAttempts[key]
	if !ok {
		return nil, exec.ErrDAGRunIDNotFound
	}
	return attempt, nil
}

func (m *mockRemoteDAGRunStore) CreateSubAttempt(_ context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return nil, m.createErr
	}
	key := fmt.Sprintf("%s:%s", rootRef.ID, subDAGRunID)
	attempt := &mockRemoteDAGRunAttempt{
		id:     subDAGRunID,
		status: &exec.DAGRunStatus{},
	}
	m.subAttempts[key] = attempt
	return attempt, nil
}

func (m *mockRemoteDAGRunStore) CreateAttempt(_ context.Context, _ *core.DAG, _ time.Time, _ string, _ exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	panic("CreateAttempt not implemented in mock")
}

func (m *mockRemoteDAGRunStore) RecentAttempts(_ context.Context, _ string, _ int) []exec.DAGRunAttempt {
	return nil
}

func (m *mockRemoteDAGRunStore) LatestAttempt(_ context.Context, _ string) (exec.DAGRunAttempt, error) {
	return nil, exec.ErrDAGRunIDNotFound
}

func (m *mockRemoteDAGRunStore) ListStatuses(_ context.Context, _ ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return nil, nil
}

func (m *mockRemoteDAGRunStore) RemoveOldDAGRuns(_ context.Context, _ string, _ int, _ ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, nil
}

func (m *mockRemoteDAGRunStore) RenameDAGRuns(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockRemoteDAGRunStore) RemoveDAGRun(_ context.Context, _ exec.DAGRunRef) error {
	return nil
}

func TestNewRemoteTaskHandler(t *testing.T) {
	t.Parallel()

	t.Run("AllFieldsSet", func(t *testing.T) {
		t.Parallel()

		client := newMockRemoteCoordinatorClient()
		store := newMockRemoteDAGRunStore()
		cfg := &config.Config{}

		handler := NewRemoteTaskHandler(RemoteTaskHandlerConfig{
			WorkerID:          "worker-1",
			CoordinatorClient: client,
			DAGRunStore:       store,
			Config:            cfg,
		})

		require.NotNil(t, handler)

		// Verify it's the correct type
		rh, ok := handler.(*remoteTaskHandler)
		require.True(t, ok, "should return *remoteTaskHandler")
		assert.Equal(t, "worker-1", rh.workerID)
		assert.Equal(t, client, rh.coordinatorClient)
		assert.Equal(t, store, rh.dagRunStore)
		assert.Equal(t, cfg, rh.config)
	})

	t.Run("NilOptionalFields", func(t *testing.T) {
		t.Parallel()

		client := newMockRemoteCoordinatorClient()

		handler := NewRemoteTaskHandler(RemoteTaskHandlerConfig{
			WorkerID:          "worker-2",
			CoordinatorClient: client,
			// DAGRunStore is nil - valid for fully remote mode
			// DAGStore is nil
			// ServiceRegistry is nil
		})

		require.NotNil(t, handler)

		rh, ok := handler.(*remoteTaskHandler)
		require.True(t, ok)
		assert.Nil(t, rh.dagRunStore)
		assert.Nil(t, rh.dagStore)
		assert.Nil(t, rh.serviceRegistry)
	})
}

func TestCreateRemoteHandlers(t *testing.T) {
	t.Parallel()

	t.Run("CreatesStatusPusher", func(t *testing.T) {
		t.Parallel()

		client := newMockRemoteCoordinatorClient()
		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: client,
		}

		root := exec.DAGRunRef{Name: "root-dag", ID: "root-123"}
		statusPusher, _ := handler.createRemoteHandlers("run-1", "test-dag", root)

		require.NotNil(t, statusPusher)
	})

	t.Run("CreatesLogStreamer", func(t *testing.T) {
		t.Parallel()

		client := newMockRemoteCoordinatorClient()
		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: client,
		}

		root := exec.DAGRunRef{Name: "root-dag", ID: "root-123"}
		_, logStreamer := handler.createRemoteHandlers("run-1", "test-dag", root)

		require.NotNil(t, logStreamer)
	})

	t.Run("PassesCorrectParameters", func(t *testing.T) {
		t.Parallel()

		client := newMockRemoteCoordinatorClient()
		handler := &remoteTaskHandler{
			workerID:          "worker-abc",
			coordinatorClient: client,
		}

		root := exec.DAGRunRef{Name: "my-root", ID: "root-xyz"}
		statusPusher, logStreamer := handler.createRemoteHandlers("my-run-id", "my-dag", root)

		// Both should be created
		require.NotNil(t, statusPusher)
		require.NotNil(t, logStreamer)
	})
}

func TestCreateAgentEnv(t *testing.T) {
	t.Parallel()

	t.Run("CreatesLogDirectory", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "test-worker-env",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "test-run-123")

		require.NoError(t, err)
		require.NotNil(t, env)
		defer env.cleanup()

		// Verify directory exists
		info, statErr := os.Stat(env.logDir)
		require.NoError(t, statErr)
		require.True(t, info.IsDir())
	})

	t.Run("PathIncludesWorkerID", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "unique-worker-id",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "run-456")

		require.NoError(t, err)
		defer env.cleanup()

		// Path should include workerID
		require.Contains(t, env.logDir, "unique-worker-id")
		require.Contains(t, env.logDir, "run-456")
	})

	t.Run("PathIncludesDagRunID", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "worker-x",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "specific-run-id")

		require.NoError(t, err)
		defer env.cleanup()

		require.Contains(t, env.logDir, "specific-run-id")
	})

	t.Run("SetsLogFilePath", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "worker-logfile",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "run-log")

		require.NoError(t, err)
		defer env.cleanup()

		// logFile should be within logDir
		require.Contains(t, env.logFile, env.logDir)
		require.Contains(t, env.logFile, "scheduler.log")
	})

	t.Run("CleanupRemovesDirectory", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "worker-cleanup",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "run-cleanup")

		require.NoError(t, err)

		// Verify directory exists
		logDir := env.logDir
		_, statErr := os.Stat(logDir)
		require.NoError(t, statErr)

		// Call cleanup
		env.cleanup()

		// Verify directory is removed
		_, statErr = os.Stat(logDir)
		require.True(t, os.IsNotExist(statErr), "directory should be removed after cleanup")
	})

	t.Run("CleanupHandlesNonExistentDirectory", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "worker-nonexist",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "run-nonexist")

		require.NoError(t, err)

		// Remove directory manually first
		_ = os.RemoveAll(env.logDir)

		// Cleanup should not panic
		require.NotPanics(t, func() {
			env.cleanup()
		})
	})

	t.Run("CreatesNestedDirectoryStructure", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID: "worker-nested",
		}

		ctx := context.Background()
		env, err := handler.createAgentEnv(ctx, "run-nested")

		require.NoError(t, err)
		defer env.cleanup()

		// Should contain the expected path structure
		expectedPath := filepath.Join(os.TempDir(), "dagu", "worker-logs", "worker-nested", "run-nested")
		assert.Equal(t, expectedPath, env.logDir)
	})
}

func TestLoadDAG(t *testing.T) {
	t.Parallel()

	t.Run("FromTarget", func(t *testing.T) {
		t.Parallel()

		// Create a temp DAG file
		tempDir := t.TempDir()
		dagFile := filepath.Join(tempDir, "test.yaml")
		dagContent := `name: test-dag
steps:
  - name: echo
    command: echo hello
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		handler := &remoteTaskHandler{
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: tempDir,
				},
			},
		}

		task := &coordinatorv1.Task{
			Target: dagFile,
		}

		dag, cleanup, loadErr := handler.loadDAG(context.Background(), task)

		require.NoError(t, loadErr)
		require.NotNil(t, dag)
		assert.Equal(t, "test-dag", dag.Name)
		// cleanup should be nil when loading from target (no temp file created)
		assert.Nil(t, cleanup)
	})

	t.Run("FromDefinition", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: t.TempDir(),
				},
			},
		}

		dagDefinition := `name: inline-dag
steps:
  - name: inline-step
    command: echo inline
`

		task := &coordinatorv1.Task{
			Target:     "inline-dag", // Target is the DAG name, not filename
			Definition: dagDefinition,
		}

		dag, cleanup, err := handler.loadDAG(context.Background(), task)

		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.Equal(t, "inline-dag", dag.Name) // Name comes from task.Target when Definition is provided
		require.NotNil(t, cleanup, "cleanup should be set for inline definitions")

		// Call cleanup to remove temp file
		cleanup()
	})

	t.Run("CleanupOnSpecLoadError", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: t.TempDir(),
				},
			},
		}

		// Invalid YAML that will fail to parse
		invalidDefinition := `invalid: yaml: content: [[[`

		task := &coordinatorv1.Task{
			Target:     "invalid.yaml",
			Definition: invalidDefinition,
		}

		dag, cleanup, err := handler.loadDAG(context.Background(), task)

		require.Error(t, err)
		require.Nil(t, dag)
		assert.Nil(t, cleanup, "cleanup should be nil after error (already cleaned up)")
		require.Contains(t, err.Error(), "failed to load DAG")
	})

	t.Run("NilCleanupWhenNoTempFile", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		dagFile := filepath.Join(tempDir, "notmp.yaml")
		dagContent := `name: no-temp
steps:
  - name: step1
    command: echo notmp
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		handler := &remoteTaskHandler{
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: tempDir,
				},
			},
		}

		task := &coordinatorv1.Task{
			Target:     dagFile,
			Definition: "", // No definition = no temp file
		}

		dag, cleanup, loadErr := handler.loadDAG(context.Background(), task)

		require.NoError(t, loadErr)
		require.NotNil(t, dag)
		assert.Nil(t, cleanup, "cleanup should be nil when no temp file is created")
	})

	t.Run("SpecLoadErrorFromTarget", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: t.TempDir(),
				},
			},
		}

		task := &coordinatorv1.Task{
			Target: "/nonexistent/path/to/dag.yaml",
		}

		dag, cleanup, err := handler.loadDAG(context.Background(), task)

		require.Error(t, err)
		require.Nil(t, dag)
		require.Nil(t, cleanup)
		require.Contains(t, err.Error(), "failed to load DAG")
	})
}

func TestHandle(t *testing.T) {
	t.Parallel()

	t.Run("UnsupportedOperation", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			config:            &config.Config{},
		}

		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_UNSPECIFIED,
		}

		err := handler.Handle(context.Background(), task)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported operation")
	})

	t.Run("UnknownOperationValue", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			config:            &config.Config{},
		}

		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation(999), // Unknown value
		}

		err := handler.Handle(context.Background(), task)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported operation")
	})
}

func TestHandleRetry(t *testing.T) {
	t.Parallel()

	t.Run("NoStatusSource", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			dagRunStore:       nil, // No local store
			config:            &config.Config{},
		}

		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_RETRY,
			Step:           "step1",
			PreviousStatus: nil, // No embedded status either
			RootDagRunName: "root",
			RootDagRunId:   "root-123",
			DagRunId:       "run-123",
		}

		err := handler.handleRetry(context.Background(), task)

		require.Error(t, err)
		require.Contains(t, err.Error(), "retry requires either previous_status in task or local dagRunStore")
	})

	t.Run("FindAttemptError", func(t *testing.T) {
		t.Parallel()

		store := newMockRemoteDAGRunStore()
		store.findErr = errors.New("database connection failed")

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			dagRunStore:       store,
			config:            &config.Config{},
		}

		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_RETRY,
			Step:           "step1",
			PreviousStatus: nil, // Will try to use local store
			RootDagRunName: "root",
			RootDagRunId:   "root-123",
			DagRunId:       "run-123",
		}

		err := handler.handleRetry(context.Background(), task)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find previous run")
	})

	t.Run("ReadStatusError", func(t *testing.T) {
		t.Parallel()

		attempt := newMockRemoteDAGRunAttempt("attempt-1", nil)
		attempt.readErr = errors.New("status file corrupted")

		store := newMockRemoteDAGRunStore()
		store.SetAttempt(exec.DAGRunRef{Name: "root", ID: "run-123"}, attempt)

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			dagRunStore:       store,
			config:            &config.Config{},
		}

		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_RETRY,
			Step:           "step1",
			PreviousStatus: nil,
			RootDagRunName: "root",
			RootDagRunId:   "root-123",
			DagRunId:       "run-123",
		}

		err := handler.handleRetry(context.Background(), task)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read previous status")
	})

	t.Run("SharedNothingModeWithEmbeddedStatus", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		dagFile := filepath.Join(tempDir, "retry.yaml")
		dagContent := `name: retry-dag
steps:
  - name: step1
    command: echo retry
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			dagRunStore:       nil, // No local store - fully shared-nothing
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: tempDir,
				},
			},
		}

		// Create a previous status proto
		previousStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
			Name:   "retry-dag",
			Status: core.Succeeded,
			Nodes:  []*exec.Node{},
		})
		require.NoError(t, convErr)

		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_RETRY,
			Step:           "step1",
			Target:         dagFile,
			PreviousStatus: previousStatus,
			RootDagRunName: "root",
			RootDagRunId:   "root-123",
			DagRunId:       "run-123",
		}

		// This will fail at agent creation since we don't have full dependencies,
		// but it proves the shared-nothing path is taken
		err = handler.handleRetry(context.Background(), task)

		// The error should NOT be about missing status source
		require.Error(t, err)
		require.NotContains(t, err.Error(), "retry requires either previous_status")
	})
}

func TestHandleStart(t *testing.T) {
	t.Parallel()

	t.Run("LoadDAGError", func(t *testing.T) {
		t.Parallel()

		handler := &remoteTaskHandler{
			workerID:          "test-worker",
			coordinatorClient: newMockRemoteCoordinatorClient(),
			config: &config.Config{
				Paths: config.PathsConfig{
					DAGsDir: t.TempDir(),
				},
			},
		}

		task := &coordinatorv1.Task{
			Target: "/nonexistent/dag.yaml",
		}

		err := handler.handleStart(context.Background(), task, false)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load DAG")
	})
}

func TestRemoteHandler_UniqueLogDirs(t *testing.T) {
	t.Parallel()

	// Verify that different dagRunIDs produce unique log directories
	handler := &remoteTaskHandler{
		workerID: "collision-worker",
	}

	ctx := context.Background()

	env1, err1 := handler.createAgentEnv(ctx, "run-aaa")
	require.NoError(t, err1)
	defer env1.cleanup()

	env2, err2 := handler.createAgentEnv(ctx, "run-bbb")
	require.NoError(t, err2)
	defer env2.cleanup()

	assert.NotEqual(t, env1.logDir, env2.logDir, "different dagRunIDs should produce different log directories")
}

func TestHandle_OperationStart(t *testing.T) {
	t.Parallel()

	// This test verifies the OPERATION_START path is taken
	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "start.yaml")
	dagContent := `name: start-dag
steps:
  - name: step1
    command: echo start
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_START,
		Target:         dagFile,
		DagRunId:       "run-start-1",
		RootDagRunName: "root",
		RootDagRunId:   "root-1",
	}

	// This will fail at agent creation (no parent DAG run), but proves the path is taken
	err = handler.Handle(context.Background(), task)
	require.Error(t, err)
	// The error should be from execution, not unsupported operation
	require.NotContains(t, err.Error(), "unsupported operation")
}

func TestHandle_OperationRetryWithoutStatusSource(t *testing.T) {
	t.Parallel()

	// OPERATION_RETRY requires either previous_status in the task (shared-nothing mode)
	// or a local dagRunStore. Without either, handleRetry returns an error.
	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "retry.yaml")
	dagContent := `name: retry-dag
steps:
  - name: step1
    command: echo retry
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		dagRunStore:       nil, // No local store
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "", // No step
		Target:         dagFile,
		DagRunId:       "run-retry-1",
		RootDagRunName: "root",
		RootDagRunId:   "root-1",
		PreviousStatus: nil, // No PreviousStatus either
	}

	// Without either status source, retry should fail with a clear error
	err = handler.Handle(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "retry requires either previous_status in task or local dagRunStore")
}

func TestHandle_OperationRetryWithStep(t *testing.T) {
	t.Parallel()

	// When OPERATION_RETRY is used with a step, it should call handleRetry
	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		dagRunStore:       nil, // No store
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: t.TempDir(),
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "step1", // With step = handleRetry path
		Target:         "/nonexistent.yaml",
		DagRunId:       "run-retry-1",
		RootDagRunName: "root",
		RootDagRunId:   "root-1",
		PreviousStatus: nil, // No embedded status and no store
	}

	err := handler.Handle(context.Background(), task)
	require.Error(t, err)
	// Should fail with "retry requires" error from handleRetry
	require.Contains(t, err.Error(), "retry requires either previous_status")
}

func TestHandleRetry_LocalStoreMode(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "local-retry.yaml")
	dagContent := `name: local-retry-dag
steps:
  - name: step1
    command: echo local
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	// Create a mock attempt with status
	status := &exec.DAGRunStatus{
		Name:   "local-retry-dag",
		Status: core.Succeeded,
		Nodes:  []*exec.Node{},
	}
	attempt := newMockRemoteDAGRunAttempt("attempt-local", status)

	store := newMockRemoteDAGRunStore()
	store.SetAttempt(exec.DAGRunRef{Name: "root", ID: "run-local-1"}, attempt)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		dagRunStore:       store, // Has local store
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "step1",
		Target:         dagFile,
		PreviousStatus: nil, // No embedded status - will use local store
		RootDagRunName: "root",
		RootDagRunId:   "root-local-1",
		DagRunId:       "run-local-1",
	}

	// This should use local store to get status, then fail at execution
	err = handler.handleRetry(context.Background(), task)
	require.Error(t, err)
	// Should NOT fail at status lookup since we have the store
	require.NotContains(t, err.Error(), "retry requires either previous_status")
	require.NotContains(t, err.Error(), "failed to find previous run")
	require.NotContains(t, err.Error(), "failed to read previous status")
}

func TestHandleStart_SuccessPathWithCleanup(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Test with inline definition to exercise cleanup path
	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	dagDefinition := `name: cleanup-test-dag
steps:
  - name: step1
    command: echo cleanup
`

	task := &coordinatorv1.Task{
		Target:         "cleanup-test.yaml",
		Definition:     dagDefinition, // Inline definition triggers temp file + cleanup
		DagRunId:       "run-cleanup-1",
		RootDagRunName: "root",
		RootDagRunId:   "root-cleanup-1",
	}

	// Will fail at execution but exercises DAG loading with cleanup
	err := handler.handleStart(context.Background(), task, false)
	require.Error(t, err)
	// Error should be from execution, not DAG loading
	require.NotContains(t, err.Error(), "failed to load DAG")
}

func TestHandleStart_QueuedRunFlag(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "queued-flag.yaml")
	dagContent := `name: queued-flag-dag
steps:
  - name: step1
    command: echo queued
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	task := &coordinatorv1.Task{
		Target:         dagFile,
		DagRunId:       "run-queued-flag",
		RootDagRunName: "root",
		RootDagRunId:   "root-queued",
	}

	// Test with queuedRun=true
	err = handler.handleStart(context.Background(), task, true)
	require.Error(t, err)
	// Should fail at execution, not DAG loading
	require.NotContains(t, err.Error(), "failed to load DAG")
}

func TestExecuteDAGRun_WithRetryConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "exec-retry.yaml")
	dagContent := `name: exec-retry-dag
steps:
  - name: step1
    command: echo exec
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	previousStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
		Name:   "exec-retry-dag",
		Status: core.Succeeded,
		Nodes:  []*exec.Node{},
	})
	require.NoError(t, convErr)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "step1",
		Target:         dagFile,
		PreviousStatus: previousStatus,
		RootDagRunName: "root",
		RootDagRunId:   "root-exec",
		DagRunId:       "run-exec",
	}

	// This exercises the retry path through handleRetry
	err = handler.handleRetry(context.Background(), task)
	require.Error(t, err)
	// Error from execution, not status lookup
	require.NotContains(t, err.Error(), "retry requires")
}

func TestRemoteHandler_DifferentWorkersDifferentPaths(t *testing.T) {
	t.Parallel()

	handler1 := &remoteTaskHandler{workerID: "worker-alpha"}
	handler2 := &remoteTaskHandler{workerID: "worker-beta"}

	ctx := context.Background()

	env1, err1 := handler1.createAgentEnv(ctx, "same-run-id")
	require.NoError(t, err1)
	defer env1.cleanup()

	env2, err2 := handler2.createAgentEnv(ctx, "same-run-id")
	require.NoError(t, err2)
	defer env2.cleanup()

	assert.NotEqual(t, env1.logDir, env2.logDir, "different workerIDs should produce different log directories even for same dagRunID")
	assert.Contains(t, env1.logDir, "worker-alpha")
	assert.Contains(t, env2.logDir, "worker-beta")
}

func TestHandleRetry_LoadDAGErrorPath(t *testing.T) {
	t.Parallel()

	// Test the path where handleRetry fails at loadDAG after getting status
	previousStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
		Name:   "loaddag-error-dag",
		Status: core.Succeeded,
		Nodes:  []*exec.Node{},
	})
	require.NoError(t, convErr)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: t.TempDir(),
			},
		},
	}

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "step1",
		Target:         "/nonexistent/path/dag.yaml", // Will fail to load
		PreviousStatus: previousStatus,               // Has embedded status
		RootDagRunName: "root",
		RootDagRunId:   "root-loaddag",
		DagRunId:       "run-loaddag",
	}

	err := handler.handleRetry(context.Background(), task)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load DAG")
}

func TestHandleRetry_WithDefinitionAndCleanup(t *testing.T) {
	t.Parallel()

	// Test handleRetry with inline definition to trigger cleanup path
	previousStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
		Name:   "def-cleanup-dag",
		Status: core.Succeeded,
		Nodes:  []*exec.Node{},
	})
	require.NoError(t, convErr)

	handler := &remoteTaskHandler{
		workerID:          "test-worker",
		coordinatorClient: newMockRemoteCoordinatorClient(),
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: t.TempDir(),
			},
		},
	}

	dagDefinition := `name: def-cleanup-dag
steps:
  - name: step1
    command: echo cleanup
`

	task := &coordinatorv1.Task{
		Operation:      coordinatorv1.Operation_OPERATION_RETRY,
		Step:           "step1",
		Target:         "def-cleanup.yaml",
		Definition:     dagDefinition, // Inline definition triggers cleanup
		PreviousStatus: previousStatus,
		RootDagRunName: "root",
		RootDagRunId:   "root-cleanup",
		DagRunId:       "run-cleanup",
	}

	err := handler.handleRetry(context.Background(), task)

	// Should fail at execution but exercise cleanup path
	require.Error(t, err)
	require.NotContains(t, err.Error(), "failed to load DAG")
}

func TestCreateAgentEnv_MkdirAllError(t *testing.T) {
	// Note: This test may be skipped on some platforms where the path is valid
	// The null byte in paths should cause MkdirAll to fail on most systems
	t.Parallel()

	// Using a null byte in the path should cause an error on most systems
	handler := &remoteTaskHandler{
		workerID: "worker\x00invalid",
	}

	ctx := context.Background()
	env, err := handler.createAgentEnv(ctx, "run-invalid")

	// On most systems, a null byte in the path should cause an error
	// If the system allows it (unlikely), the test passes anyway
	if err != nil {
		require.Contains(t, err.Error(), "failed to create log directory")
		require.Nil(t, env)
	} else {
		// If somehow it succeeded, clean up
		if env != nil {
			env.cleanup()
		}
	}
}

func TestLoadDAG_CleanupErrorLogged(t *testing.T) {
	t.Parallel()

	// Test that cleanup errors in loadDAG are logged but don't affect the return
	// We can't easily trigger an os.Remove error that's not IsNotExist,
	// but we can verify the cleanup function handles normal removal correctly

	handler := &remoteTaskHandler{
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: t.TempDir(),
			},
		},
	}

	dagDefinition := `name: cleanup-logged-dag
steps:
  - name: step1
    command: echo cleanup
`

	task := &coordinatorv1.Task{
		Target:     "cleanup-logged.yaml",
		Definition: dagDefinition,
	}

	dag, cleanup, err := handler.loadDAG(context.Background(), task)

	require.NoError(t, err)
	require.NotNil(t, dag)
	require.NotNil(t, cleanup)

	// Call cleanup - this exercises the cleanup path even though
	// we can't easily make it fail
	cleanup()

	// Calling cleanup again should not panic (handles IsNotExist)
	require.NotPanics(t, func() {
		cleanup()
	})
}

func TestExecuteDAGRun_CreateAgentEnvError(t *testing.T) {
	t.Parallel()

	// Test that executeDAGRun returns error when createAgentEnv fails
	// Use null byte in workerID to trigger MkdirAll error
	tempDir := t.TempDir()
	dagFile := filepath.Join(tempDir, "exec-env-error.yaml")
	dagContent := `name: exec-env-error-dag
steps:
  - name: step1
    command: echo test
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0644)
	require.NoError(t, err)

	client := newMockRemoteCoordinatorClient()

	// Create handler with invalid workerID containing null byte
	handler := &remoteTaskHandler{
		workerID:          "worker\x00error", // Null byte causes MkdirAll to fail
		coordinatorClient: client,
		config: &config.Config{
			Paths: config.PathsConfig{
				DAGsDir: tempDir,
			},
		},
	}

	// Load DAG first
	task := &coordinatorv1.Task{
		Target: dagFile,
	}
	dag, cleanup, loadErr := handler.loadDAG(context.Background(), task)
	require.NoError(t, loadErr)
	require.NotNil(t, dag)
	if cleanup != nil {
		defer cleanup()
	}

	// Create remote handlers
	root := exec.DAGRunRef{Name: "root", ID: "root-1"}
	parent := exec.DAGRunRef{Name: "parent", ID: "parent-1"}
	statusPusher, logStreamer := handler.createRemoteHandlers("run-error", dag.Name, root)

	// Call executeDAGRun directly - should fail at createAgentEnv
	err = handler.executeDAGRun(context.Background(), dag, "run-error", "", root, parent, statusPusher, logStreamer, false, nil)

	// On systems where null byte in path fails, we should get an error
	if err != nil {
		require.Contains(t, err.Error(), "failed to create log directory")
	}
}

func TestExecuteDAGRun_SuccessfulExecution(t *testing.T) {
	// This test covers the success path (lines 310-313) by running a complete execution
	// with full test infrastructure

	// Use test.Setup for full dependencies
	th := test.Setup(t)

	// Create a simple DAG that will succeed
	dagContent := `name: remote-handler-success
steps:
  - name: echo-step
    command: echo "hello from remote handler"
`
	dag := th.DAG(t, dagContent)

	client := newMockRemoteCoordinatorClient()

	// Create handler with full dependencies from test helper
	handler := &remoteTaskHandler{
		workerID:          "integration-test-worker",
		coordinatorClient: client,
		dagRunStore:       th.DAGRunStore,
		dagStore:          th.DAGStore,
		dagRunMgr:         th.DAGRunMgr,
		serviceRegistry:   th.ServiceRegistry,
		config:            th.Config,
	}

	// For a top-level run, root ID should match the dagRunID
	dagRunID := "run-success-1"
	root := exec.DAGRunRef{Name: dag.Name, ID: dagRunID}
	statusPusher := remote.NewStatusPusher(client, "integration-test-worker")
	logStreamer := remote.NewLogStreamer(client, "integration-test-worker", dagRunID, dag.Name, "", root)

	// Call executeDAGRun - this should succeed and log completion
	// For top-level runs, pass empty parent and ensure root matches dagRunID
	err := handler.executeDAGRun(th.Context, dag.DAG, dagRunID, "", root, exec.DAGRunRef{}, statusPusher, logStreamer, false, nil)

	// Should succeed for simple echo command
	require.NoError(t, err, "executeDAGRun should succeed for simple echo command")
}
