package metrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/builder"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
)

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

func (m *mockDAGStore) List(ctx context.Context, params models.ListDAGsOptions) (models.PaginatedResult[*digraph.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(models.PaginatedResult[*digraph.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, fileName string) (*digraph.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *mockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...builder.LoadOption) (*digraph.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*models.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*models.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
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

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...builder.LoadOption) (*digraph.DAG, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
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

var _ models.DAGRunStore = (*mockDAGRunStore)(nil)

type mockDAGRunStore struct {
	mock.Mock
}

// RemoveDAGRun implements models.DAGRunStore.
func (m *mockDAGRunStore) RemoveDAGRun(_ context.Context, _ digraph.DAGRunRef) error {
	panic("unimplemented")
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *digraph.DAG, ts time.Time, dagRunID string, opts models.NewDAGRunAttemptOptions) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []models.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]models.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) ListStatuses(ctx context.Context, opts ...models.ListDAGRunStatusesOption) ([]*models.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunStore) FindAttempt(ctx context.Context, dagRun digraph.DAGRunRef) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) FindChildAttempt(ctx context.Context, dagRun digraph.DAGRunRef, childDAGRunID string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun, childDAGRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int) error {
	args := m.Called(ctx, name, retentionDays)
	return args.Error(0)
}

func (m *mockDAGRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	args := m.Called(ctx, oldName, newName)
	return args.Error(0)
}

var _ models.QueueStore = (*mockQueueStore)(nil)

type mockQueueStore struct {
	mock.Mock
}

// ListByDAGName implements models.QueueStore.
func (m *mockQueueStore) ListByDAGName(_ context.Context, _, _ string) ([]models.QueuedItemData, error) {
	return nil, nil
}

func (m *mockQueueStore) Enqueue(ctx context.Context, name string, priority models.QueuePriority, dagRun digraph.DAGRunRef) error {
	args := m.Called(ctx, name, priority, dagRun)
	return args.Error(0)
}

func (m *mockQueueStore) DequeueByName(ctx context.Context, name string) (models.QueuedItemData, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) DequeueByDAGRunID(ctx context.Context, name, dagRunID string) ([]models.QueuedItemData, error) {
	args := m.Called(ctx, name, dagRunID)
	return args.Get(0).([]models.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) Len(ctx context.Context, name string) (int, error) {
	args := m.Called(ctx, name)
	return args.Int(0), args.Error(1)
}

func (m *mockQueueStore) List(ctx context.Context, name string) ([]models.QueuedItemData, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]models.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) All(ctx context.Context) ([]models.QueuedItemData, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QueuedItemData), args.Error(1)
}

func (m *mockQueueStore) Reader(ctx context.Context) models.QueueReader {
	args := m.Called(ctx)
	return args.Get(0).(models.QueueReader)
}

type mockServiceRegistry struct {
	mock.Mock
}

func (m *mockServiceRegistry) Register(ctx context.Context, serviceName models.ServiceName, hostInfo models.HostInfo) error {
	args := m.Called(ctx, serviceName, hostInfo)
	return args.Error(0)
}

func (m *mockServiceRegistry) Unregister(ctx context.Context) {
	m.Called(ctx)
}

func (m *mockServiceRegistry) GetServiceMembers(ctx context.Context, serviceName models.ServiceName) ([]models.HostInfo, error) {
	args := m.Called(ctx, serviceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.HostInfo), args.Error(1)
}

func (m *mockServiceRegistry) UpdateStatus(ctx context.Context, serviceName models.ServiceName, status models.ServiceStatus) error {
	args := m.Called(ctx, serviceName, status)
	return args.Error(0)
}

// Tests

func TestNewCollector(t *testing.T) {
	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return([]models.HostInfo{{Host: "localhost", Status: models.ServiceStatusActive}}, nil)

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

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	// Count the number of metrics described
	count := 0
	for range ch {
		count++
	}

	// We should have 7 metrics described
	assert.Equal(t, 7, count)
}

func TestCollector_Collect_BasicMetrics(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	// Setup mocks to return empty data
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		models.PaginatedResult[*digraph.DAG]{},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*models.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]models.QueuedItemData{}, nil)

	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return([]models.HostInfo{{Host: "localhost", Status: models.ServiceStatusActive}}, nil).Maybe()

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		serviceRegistry,
	)

	// Test uptime metric (should be > 0)
	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	// Should have at least the basic metrics
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
		models.PaginatedResult[*digraph.DAG]{
			Items:      []*digraph.DAG{{}, {}, {}},
			TotalCount: 3,
		},
		[]string{},
		nil,
	)

	// Mock DAG run store response
	statuses := []*models.DAGRunStatus{
		{Status: status.Success},
		{Status: status.Success},
		{Status: status.Error},
		{Status: status.Running},
		{Status: status.Queued},
		{Status: status.Cancel},
	}
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return(statuses, nil)

	// Mock queue store response
	queueStore.On("All", mock.Anything).Return([]models.QueuedItemData{nil, nil}, nil)

	serviceRegistry := &mockServiceRegistry{}
	serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return([]models.HostInfo{{Host: "localhost", Status: models.ServiceStatusActive}}, nil).Maybe()

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
				case "success":
					assert.Equal(t, float64(2), *metric.Counter.Value)
				case "error":
					assert.Equal(t, float64(1), *metric.Counter.Value)
				case "cancelled":
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

	// Mock errors
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		models.PaginatedResult[*digraph.DAG]{},
		[]string{},
		assert.AnError,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*models.DAGRunStatus(nil), assert.AnError)
	queueStore.On("All", mock.Anything).Return([]models.QueuedItemData(nil), assert.AnError)

	collector := NewCollector(
		"1.0.0",
		dagStore,
		dagRunStore,
		queueStore,
		nil,
	)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	// Should still collect basic metrics even with errors
	metricsCount := 0
	for range ch {
		metricsCount++
	}
	assert.Greater(t, metricsCount, 0) // At least info, uptime, scheduler
}

func TestNewRegistry(t *testing.T) {
	dagStore := &mockDAGStore{}
	dagRunStore := &mockDAGRunStore{}
	queueStore := &mockQueueStore{}

	// Setup mocks
	dagStore.On("List", mock.Anything, mock.Anything).Return(
		models.PaginatedResult[*digraph.DAG]{},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*models.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]models.QueuedItemData{}, nil)

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
		models.PaginatedResult[*digraph.DAG]{Items: []*digraph.DAG{}, TotalCount: 0},
		[]string{},
		nil,
	)
	dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*models.DAGRunStatus{}, nil)
	queueStore.On("All", mock.Anything).Return([]models.QueuedItemData{}, nil)

	t.Run("ActiveScheduler", func(t *testing.T) {
		serviceRegistry := &mockServiceRegistry{}
		serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return(
			[]models.HostInfo{{Host: "localhost", Status: models.ServiceStatusActive}},
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
		serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return(
			[]models.HostInfo{{Host: "localhost", Status: models.ServiceStatusInactive}},
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
		serviceRegistry.On("GetServiceMembers", mock.Anything, models.ServiceNameScheduler).Return(
			[]models.HostInfo{},
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
