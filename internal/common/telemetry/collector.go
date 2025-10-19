package telemetry

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Collector implements prometheus.Collector interface
type Collector struct {
	startTime       time.Time
	version         string
	dagStore        execution.DAGStore
	dagRunStore     execution.DAGRunStore
	queueStore      execution.QueueStore
	serviceRegistry execution.ServiceRegistry

	// Metric descriptors
	infoDesc             *prometheus.Desc
	uptimeDesc           *prometheus.Desc
	dagRunsCurrentlyDesc *prometheus.Desc
	dagRunsQueuedDesc    *prometheus.Desc
	dagRunsTotalDesc     *prometheus.Desc
	dagsTotalDesc        *prometheus.Desc
	schedulerRunningDesc *prometheus.Desc

	mu sync.RWMutex
}

// NewCollector creates a new metrics collector
func NewCollector(
	version string,
	dagStore execution.DAGStore,
	dagRunStore execution.DAGRunStore,
	queueStore execution.QueueStore,
	serviceRegistry execution.ServiceRegistry,
) *Collector {
	return &Collector{
		startTime:       time.Now(),
		version:         version,
		dagStore:        dagStore,
		dagRunStore:     dagRunStore,
		queueStore:      queueStore,
		serviceRegistry: serviceRegistry,

		// Initialize metric descriptors
		infoDesc: prometheus.NewDesc(
			"dagu_info",
			"Dagu build information",
			[]string{"version", "go_version"},
			nil,
		),
		uptimeDesc: prometheus.NewDesc(
			"dagu_uptime_seconds",
			"Time since server start",
			nil,
			nil,
		),
		dagRunsCurrentlyDesc: prometheus.NewDesc(
			"dagu_dag_runs_currently_running",
			"Number of currently running DAG runs",
			nil,
			nil,
		),
		dagRunsQueuedDesc: prometheus.NewDesc(
			"dagu_dag_runs_queued_total",
			"Total number of DAG runs in queue",
			nil,
			nil,
		),
		dagRunsTotalDesc: prometheus.NewDesc(
			"dagu_dag_runs_total",
			"Total number of DAG runs by status (last 24 hours)",
			[]string{"status"},
			nil,
		),
		dagsTotalDesc: prometheus.NewDesc(
			"dagu_dags_total",
			"Total number of DAGs",
			nil,
			nil,
		),
		schedulerRunningDesc: prometheus.NewDesc(
			"dagu_scheduler_running",
			"Whether the scheduler is running",
			nil,
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.infoDesc
	ch <- c.uptimeDesc
	ch <- c.dagRunsCurrentlyDesc
	ch <- c.dagRunsQueuedDesc
	ch <- c.dagRunsTotalDesc
	ch <- c.dagsTotalDesc
	ch <- c.schedulerRunningDesc
}

// Collect implements prometheus.Collector
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a context with timeout for metrics collection
	// This prevents metrics collection from hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// System info
	ch <- prometheus.MustNewConstMetric(
		c.infoDesc,
		prometheus.GaugeValue,
		1,
		c.version,
		runtime.Version(),
	)

	// Uptime
	ch <- prometheus.MustNewConstMetric(
		c.uptimeDesc,
		prometheus.GaugeValue,
		time.Since(c.startTime).Seconds(),
	)

	// Collect DAG run metrics
	c.collectDAGRunMetrics(ctx, ch)

	// Collect DAG metrics
	c.collectDAGMetrics(ctx, ch)

	// Scheduler status
	schedulerRunning := float64(0)
	if c.serviceRegistry != nil {
		members, err := c.serviceRegistry.GetServiceMembers(ctx, execution.ServiceNameScheduler)
		if err == nil {
			// Check if any scheduler instance is active
			for _, member := range members {
				if member.Status == execution.ServiceStatusActive {
					schedulerRunning = 1
					break
				}
			}
		}
	}
	ch <- prometheus.MustNewConstMetric(
		c.schedulerRunningDesc,
		prometheus.GaugeValue,
		schedulerRunning,
	)
}

func (c *Collector) collectDAGRunMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	// Get all DAG run statuses
	// NOTE: ListStatuses by default returns only the last 24 hours of data
	// This means metrics only reflect recent DAG runs, not the entire history
	statuses, err := c.dagRunStore.ListStatuses(ctx)
	if err != nil {
		// Log error but don't fail collection
		return
	}

	// Count runs by status
	statusCounts := make(map[string]float64)
	currentlyRunning := float64(0)

	for _, st := range statuses {
		if st.Status == core.Running {
			currentlyRunning++
		}

		// Use the canonical status string from String() method
		statusLabel := st.Status.String()
		statusCounts[statusLabel]++
	}

	// Currently running DAGs
	ch <- prometheus.MustNewConstMetric(
		c.dagRunsCurrentlyDesc,
		prometheus.GaugeValue,
		currentlyRunning,
	)

	// Queue length
	// NOTE: This counts all queued items across all DAGs
	// Future enhancement: Add queue name (DAG name) as a label for per-DAG queue metrics
	queuedCount := float64(0)
	if c.queueStore != nil {
		items, err := c.queueStore.All(ctx)
		if err == nil {
			queuedCount = float64(len(items))
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.dagRunsQueuedDesc,
		prometheus.GaugeValue,
		queuedCount,
	)

	// DAG runs by status
	for status, count := range statusCounts {
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsTotalDesc,
			prometheus.CounterValue,
			count,
			status,
		)
	}
}

func (c *Collector) collectDAGMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	// Get all DAGs using List with empty options to get all
	result, _, err := c.dagStore.List(ctx, execution.ListDAGsOptions{})
	if err != nil {
		// Log error but don't fail collection
		return
	}

	totalDAGs := float64(result.TotalCount)

	ch <- prometheus.MustNewConstMetric(
		c.dagsTotalDesc,
		prometheus.GaugeValue,
		totalDAGs,
	)
}

// NewRegistry creates a new Prometheus registry with Dagu collectors
func NewRegistry(collector *Collector) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	// Register custom Dagu collector
	registry.MustRegister(collector)

	// Optionally register Go runtime metrics
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	return registry
}
