package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
)

var _ execution.DAGStore = (*mockDAGStore)(nil)

// Mock implementations
type mockDAGStore struct {
	mock.Mock
}

func (m *mockDAGStore) Create(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) Delete(ctx context.Context, fileName string) error {
	args := m.Called(ctx, fileName)
	return args.Error(0)
}

func (m *mockDAGStore) List(ctx context.Context, params execution.ListDAGsOptions) (execution.PaginatedResult[*core.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(execution.PaginatedResult[*core.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, fileName string) (*core.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*execution.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*execution.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) Rename(ctx context.Context, oldID, newID string) error {
	args := m.Called(ctx, oldID, newID)
	return args.Error(0)
}

func (m *mockDAGStore) GetSpec(ctx context.Context, fileName string) (string, error) {
	args := m.Called(ctx, fileName)
	return args.String(0), args.Error(1)
}

func (m *mockDAGStore) UpdateSpec(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) TagList(ctx context.Context) ([]string, []string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) ToggleSuspend(ctx context.Context, fileName string, suspend bool) error {
	args := m.Called(ctx, fileName, suspend)
	return args.Error(0)
}

func (m *mockDAGStore) IsSuspended(ctx context.Context, fileName string) bool {
	args := m.Called(ctx, fileName)
	return args.Bool(0)
}

var _ execution.DAGRunStore = (*mockDAGRunStore)(nil)

type mockDAGRunStore struct {
	mock.Mock
}

// RemoveDAGRun implements models.DAGRunStore.
func (m *mockDAGRunStore) RemoveDAGRun(_ context.Context, _ execution.DAGRunRef) error {
	panic("unimplemented")
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts execution.NewDAGRunAttemptOptions) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []execution.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]execution.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) ListStatuses(ctx context.Context, opts ...execution.ListDAGRunStatusesOption) ([]*execution.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*execution.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunStore) FindAttempt(ctx context.Context, dagRun execution.DAGRunRef) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) FindSubAttempt(ctx context.Context, dagRun execution.DAGRunRef, subDAGRunID string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun, subDAGRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int, opts ...execution.RemoveOldDAGRunsOption) ([]string, error) {
	args := m.Called(ctx, name, retentionDays, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDAGRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	args := m.Called(ctx, oldName, newName)
	return args.Error(0)
}

var _ execution.QueueStore = (*mockQueueStore)(nil)

type mockQueueStore struct {
	mock.Mock
}

// QueueWatcher implements execution.QueueStore.
func (m *mockQueueStore) QueueWatcher(_ context.Context) execution.QueueWatcher {
	panic("unimplemented")
}

// QueueList implements execution.QueueStore.
func (m *mockQueueStore) QueueList(_ context.Context) ([]string, error) {
	panic("unimplemented")
}

// ListByDAGName implements models.QueueStore.
func (m *mockQueueStore) ListByDAGName(_ context.Context, _, _ string) ([]execution.QueuedItemData, error) {
	return nil, nil
}

func (m *mockQueueStore) Enqueue(ctx context.Context, name string, priority execution.QueuePriority, dagRun execution.DAGRunRef) error {
	args := m.Called(ctx, name, priority, dagRun)
	return args.Error(0)
}

func (m *mockQueueStore) DequeueByName(ctx context.Context, name string) (execution.QueuedItemData, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) DequeueByDAGRunID(ctx context.Context, name string, dagRun execution.DAGRunRef) ([]execution.QueuedItemData, error) {
	args := m.Called(ctx, name, dagRun)
	return args.Get(0).([]execution.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) Len(ctx context.Context, name string) (int, error) {
	args := m.Called(ctx, name)
	return args.Int(0), args.Error(1)
}

func (m *mockQueueStore) List(ctx context.Context, name string) ([]execution.QueuedItemData, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]execution.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) All(ctx context.Context) ([]execution.QueuedItemData, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]execution.QueuedItemData), args.Error(1)
}

var _ execution.ServiceRegistry = (*mockServiceRegistry)(nil)

type mockServiceRegistry struct {
	mock.Mock
}

func (m *mockServiceRegistry) Register(ctx context.Context, serviceName execution.ServiceName, hostInfo execution.HostInfo) error {
	args := m.Called(ctx, serviceName, hostInfo)
	return args.Error(0)
}

func (m *mockServiceRegistry) Unregister(ctx context.Context) {
	m.Called(ctx)
}

func (m *mockServiceRegistry) GetServiceMembers(ctx context.Context, serviceName execution.ServiceName) ([]execution.HostInfo, error) {
	args := m.Called(ctx, serviceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]execution.HostInfo), args.Error(1)
}

func (m *mockServiceRegistry) UpdateStatus(ctx context.Context, serviceName execution.ServiceName, status execution.ServiceStatus) error {
	args := m.Called(ctx, serviceName, status)
	return args.Error(0)
}

// Tests

func TestNewCollector(t *testing.T) {
	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return([]execution.HostInfo{{Host: "localhost", Status: execution.ServiceStatusActive}}, nil)

	collector := NewCollector(
		"1.0.0",
		&mockDAGStore{},
		&mockDAGRunStore{},
		&mockQueueStore{},
		serviceRegistry,
	)

	assert.NotNil(t, collector)
	assert.Equal(t, "1.0.0", collector.version)
}

func TestCollector_Describe(t *testing.T) {
	collector := NewCollector(
		"1.0.0",
		&mockDAGStore{},
		&mockDAGRunStore{},
		&mockQueueStore{},
		nil,
	)

	ch := make(chan *prometheus.Desc, 20)
	collector.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// 7 aggregate + 5 per-DAG + 1 cache metrics
	assert.Equal(t, 13, count)
}

func TestCollector_Collect_BasicMetrics(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	dagStore.On("List", mock.Anything, mock.Anything).Return(
		execution.PaginatedResult[*core.DAG]{},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*execution.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]execution.QueuedItemData{}, nil)

	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return([]execution.HostInfo{{Host: "localhost", Status: execution.ServiceStatusActive}}, nil).Maybe()

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		serviceRegistry,
	)

	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	close(ch)

	metricsCount := 0
	for range ch {
		metricsCount++
	}
	assert.Greater(t, metricsCount, 0)
}

func TestCollector_Collect_WithDAGRuns(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	// Mock DAG store response
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		execution.PaginatedResult[*core.DAG]{
			Items:      []*core.DAG{{}, {}, {}},
			TotalCount: 3,
		},
		[]string{},
		nil,
	)

	// Mock DAG run store response
	statuses := []*execution.DAGRunStatus{
		{Status: core.Succeeded},
		{Status: core.Succeeded},
		{Status: core.Failed},
		{Status: core.Running},
		{Status: core.Queued},
		{Status: core.Aborted},
	}
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return(statuses, nil)

	// Mock queue store response
	queueStore.On("All", mock.Anything).Return([]execution.QueuedItemData{nil, nil}, nil)

	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return([]execution.HostInfo{{Host: "localhost", Status: execution.ServiceStatusActive}}, nil).Maybe()

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		serviceRegistry,
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Collect metrics
	metrics, err := registry.Gather()
	assert.NoError(t, err)

	// Verify metrics
	metricMap := make(map[string]*dto.MetricFamily)
	for _, m := range metrics {
		metricMap[*m.Name] = m
	}

	// Check dagu_info
	assert.Contains(t, metricMap, "dagu_info")
	assert.Equal(t, float64(1), *metricMap["dagu_info"].Metric[0].Gauge.Value)

	// Check dagu_uptime_seconds
	assert.Contains(t, metricMap, "dagu_uptime_seconds")
	assert.Greater(t, *metricMap["dagu_uptime_seconds"].Metric[0].Gauge.Value, float64(0))

	// Check dagu_scheduler_running
	assert.Contains(t, metricMap, "dagu_scheduler_running")
	assert.Equal(t, float64(1), *metricMap["dagu_scheduler_running"].Metric[0].Gauge.Value)

	// Check dagu_dags_total
	assert.Contains(t, metricMap, "dagu_dags_total")
	assert.Equal(t, float64(3), *metricMap["dagu_dags_total"].Metric[0].Gauge.Value)

	// Check dagu_dag_runs_currently_running
	assert.Contains(t, metricMap, "dagu_dag_runs_currently_running")
	assert.Equal(t, float64(1), *metricMap["dagu_dag_runs_currently_running"].Metric[0].Gauge.Value)

	// Check dagu_dag_runs_queued_total
	assert.Contains(t, metricMap, "dagu_dag_runs_queued_total")
	assert.Equal(t, float64(2), *metricMap["dagu_dag_runs_queued_total"].Metric[0].Gauge.Value)

	// Check dagu_dag_runs_total by status
	assert.Contains(t, metricMap, "dagu_dag_runs_total")
	for _, metric := range metricMap["dagu_dag_runs_total"].Metric {
		for _, label := range metric.Label {
			if *label.Name == "status" {
				switch *label.Value {
				case "succeeded":
					assert.Equal(t, float64(2), *metric.Counter.Value)
				case "failed":
					assert.Equal(t, float64(1), *metric.Counter.Value)
				case "aborted":
					assert.Equal(t, float64(1), *metric.Counter.Value)
				case "running":
					assert.Equal(t, float64(1), *metric.Counter.Value)
				case "queued":
					assert.Equal(t, float64(1), *metric.Counter.Value)
				}
			}
		}
	}
}

func TestCollector_Collect_WithErrors(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	dagStore.On("List", mock.Anything, mock.Anything).Return(
		execution.PaginatedResult[*core.DAG]{},
		[]string{},
		assert.AnError,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*execution.DAGRunStatus(nil), assert.AnError)
	queueStore.On("All", mock.Anything).Return([]execution.QueuedItemData(nil), assert.AnError)

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		nil,
	)

	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	close(ch)

	metricsCount := 0
	for range ch {
		metricsCount++
	}
	assert.Greater(t, metricsCount, 0)
}

func TestNewRegistry(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	// Setup mocks
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		execution.PaginatedResult[*core.DAG]{},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*execution.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]execution.QueuedItemData{}, nil)

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		nil,
	)

	registry := NewRegistry(collector)
	assert.NotNil(t, registry)

	// Verify it can gather metrics without panic
	metrics, err := registry.Gather()
	assert.NoError(t, err)
	assert.Greater(t, len(metrics), 0)

	// Should include Go runtime metrics
	metricNames := make(map[string]bool)
	for _, m := range metrics {
		metricNames[*m.Name] = true
	}
	assert.True(t, metricNames["go_goroutines"]) // Example Go metric
}

func TestCollector_SchedulerStatus(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	// Set up default mock responses
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		execution.PaginatedResult[*core.DAG]{Items: []*core.DAG{}, TotalCount: 0},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*execution.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]execution.QueuedItemData{}, nil)

	t.Run("ActiveScheduler", func(t *testing.T) {
		serviceRegistry := &mockServiceRegistry{}
		serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return(
			[]execution.HostInfo{{Host: "localhost", Status: execution.ServiceStatusActive}},
			nil,
		).Maybe()

		collector := NewCollector("1.0.0", dagStore, dagRunStore, queueStore, serviceRegistry)

		ch := make(chan prometheus.Metric, 100)
		collector.Collect(ch)
		close(ch)

		// Check scheduler_running metric is 1
		schedulerRunningFound := false
		for metric := range ch {
			dto := &dto.Metric{}
			_ = metric.Write(dto)
			if strings.Contains(metric.Desc().String(), "scheduler_running") {
				schedulerRunningFound = true
				assert.Equal(t, float64(1), dto.Gauge.GetValue())
			}
		}
		assert.True(t, schedulerRunningFound, "scheduler_running metric not found")
	})

	t.Run("InactiveScheduler", func(t *testing.T) {
		serviceRegistry := &mockServiceRegistry{}
		serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return(
			[]execution.HostInfo{{Host: "localhost", Status: execution.ServiceStatusInactive}},
			nil,
		).Maybe()

		collector := NewCollector("1.0.0", dagStore, dagRunStore, queueStore, serviceRegistry)

		ch := make(chan prometheus.Metric, 100)
		collector.Collect(ch)
		close(ch)

		// Check scheduler_running metric is 0
		schedulerRunningFound := false
		for metric := range ch {
			dto := &dto.Metric{}
			_ = metric.Write(dto)
			if strings.Contains(metric.Desc().String(), "scheduler_running") {
				schedulerRunningFound = true
				assert.Equal(t, float64(0), dto.Gauge.GetValue())
			}
		}
		assert.True(t, schedulerRunningFound, "scheduler_running metric not found")
	})

	t.Run("NoSchedulerInstances", func(t *testing.T) {
		serviceRegistry := &mockServiceRegistry{}
		serviceRegistry.On("GetServiceMembers", mock.Anything, execution.ServiceNameScheduler).Return(
			[]execution.HostInfo{},
			nil,
		).Maybe()

		collector := NewCollector("1.0.0", dagStore, dagRunStore, queueStore, serviceRegistry)

		ch := make(chan prometheus.Metric, 100)
		collector.Collect(ch)
		close(ch)

		// Check scheduler_running metric is 0
		schedulerRunningFound := false
		for metric := range ch {
			dto := &dto.Metric{}
			_ = metric.Write(dto)
			if strings.Contains(metric.Desc().String(), "scheduler_running") {
				schedulerRunningFound = true
				assert.Equal(t, float64(0), dto.Gauge.GetValue())
			}
		}
		assert.True(t, schedulerRunningFound, "scheduler_running metric not found")
	})
}
