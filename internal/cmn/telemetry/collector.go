package telemetry

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// Histogram bucket definitions
var (
	// dagRunDurationBuckets defines buckets for DAG run duration (workflow-appropriate timescales)
	dagRunDurationBuckets = []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800, 3600}

	// queueWaitBuckets defines buckets for queue wait time (shorter timescales)
	queueWaitBuckets = []float64{1, 5, 10, 30, 60, 120, 300, 600}
)

// Collector implements prometheus.Collector interface
type Collector struct {
	startTime       time.Time
	version         string
	dagStore        exec.DAGStore
	dagRunStore     exec.DAGRunStore
	queueStore      exec.QueueStore
	serviceRegistry exec.ServiceRegistry
	caches          []fileutil.CacheMetrics

	// Metric descriptors (aggregate - backward compatible)
	infoDesc             *prometheus.Desc
	uptimeDesc           *prometheus.Desc
	dagRunsCurrentlyDesc *prometheus.Desc
	dagRunsQueuedDesc    *prometheus.Desc
	dagRunsTotalDesc     *prometheus.Desc
	dagsTotalDesc        *prometheus.Desc
	schedulerRunningDesc *prometheus.Desc

	// Metric descriptors (per-DAG)
	dagRunsCurrentlyByDAGDesc *prometheus.Desc
	dagRunsQueuedByDAGDesc    *prometheus.Desc
	dagRunsTotalByDAGDesc     *prometheus.Desc
	dagRunDurationDesc        *prometheus.Desc
	queueWaitTimeDesc         *prometheus.Desc

	// Cache metrics
	cacheEntriesDesc *prometheus.Desc

	mu sync.RWMutex
}

// NewCollector creates a new metrics collector
func NewCollector(
	version string,
	dagStore exec.DAGStore,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	serviceRegistry exec.ServiceRegistry,
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
			"Total number of DAG runs by status (today)",
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

		// Per-DAG metric descriptors
		dagRunsCurrentlyByDAGDesc: prometheus.NewDesc(
			"dagu_dag_runs_currently_running_by_dag",
			"Number of currently running DAG runs per DAG",
			[]string{"dag"},
			nil,
		),
		dagRunsQueuedByDAGDesc: prometheus.NewDesc(
			"dagu_dag_runs_queued_by_dag",
			"Number of queued DAG runs per DAG",
			[]string{"dag"},
			nil,
		),
		dagRunsTotalByDAGDesc: prometheus.NewDesc(
			"dagu_dag_runs_total_by_dag",
			"Total number of DAG runs by DAG and status (today)",
			[]string{"dag", "status"},
			nil,
		),
		dagRunDurationDesc: prometheus.NewDesc(
			"dagu_dag_run_duration_seconds",
			"Duration of completed DAG runs in seconds",
			[]string{"dag"},
			nil,
		),
		queueWaitTimeDesc: prometheus.NewDesc(
			"dagu_queue_wait_seconds",
			"Time spent waiting in queue before execution starts",
			[]string{"dag"},
			nil,
		),

		// Cache metrics
		cacheEntriesDesc: prometheus.NewDesc(
			"dagu_cache_entries_total",
			"Number of entries in cache",
			[]string{"cache"},
			nil,
		),
	}
}

// RegisterCache adds a cache to be monitored for metrics
func (c *Collector) RegisterCache(cache fileutil.CacheMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.caches = append(c.caches, cache)
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	// Aggregate metrics (backward compatible)
	ch <- c.infoDesc
	ch <- c.uptimeDesc
	ch <- c.dagRunsCurrentlyDesc
	ch <- c.dagRunsQueuedDesc
	ch <- c.dagRunsTotalDesc
	ch <- c.dagsTotalDesc
	ch <- c.schedulerRunningDesc

	// Per-DAG metrics
	ch <- c.dagRunsCurrentlyByDAGDesc
	ch <- c.dagRunsQueuedByDAGDesc
	ch <- c.dagRunsTotalByDAGDesc
	ch <- c.dagRunDurationDesc
	ch <- c.queueWaitTimeDesc

	// Cache metrics
	ch <- c.cacheEntriesDesc
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
		members, err := c.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameScheduler)
		if err == nil {
			// Check if any scheduler instance is active
			for _, member := range members {
				if member.Status == exec.ServiceStatusActive {
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

	// Collect cache metrics
	c.collectCacheMetrics(ch)
}

func (c *Collector) collectCacheMetrics(ch chan<- prometheus.Metric) {
	for _, cache := range c.caches {
		ch <- prometheus.MustNewConstMetric(
			c.cacheEntriesDesc,
			prometheus.GaugeValue,
			float64(cache.Size()),
			cache.Name(),
		)
	}
}

func (c *Collector) collectDAGRunMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	// Get all DAG run statuses
	// NOTE: ListStatuses by default returns only today's data (from midnight)
	statuses, err := c.dagRunStore.ListStatuses(ctx)
	if err != nil {
		return
	}

	// Aggregate metrics (backward compatible)
	statusCounts := make(map[string]float64)
	var currentlyRunning float64

	// Per-DAG aggregations
	type dagMetrics struct {
		running      float64
		statusCounts map[string]float64
		durations    []float64
		queueWaits   []float64
	}
	perDAG := make(map[string]*dagMetrics)

	for _, st := range statuses {
		dagName := st.Name
		if _, ok := perDAG[dagName]; !ok {
			perDAG[dagName] = &dagMetrics{
				statusCounts: make(map[string]float64),
			}
		}
		dm := perDAG[dagName]

		statusLabel := st.Status.String()
		statusCounts[statusLabel]++
		dm.statusCounts[statusLabel]++

		if st.Status == core.Running {
			currentlyRunning++
			dm.running++
		}

		// Collect duration for completed runs
		if isCompletedStatus(st.Status) {
			if duration := calculateDuration(st.StartedAt, st.FinishedAt); duration > 0 {
				dm.durations = append(dm.durations, duration)
			}
		}

		// Collect queue wait time
		if st.QueuedAt != "" && st.StartedAt != "" {
			if waitTime := calculateDuration(st.QueuedAt, st.StartedAt); waitTime > 0 {
				dm.queueWaits = append(dm.queueWaits, waitTime)
			}
		}
	}

	// Emit aggregate metrics (backward compatible)
	ch <- prometheus.MustNewConstMetric(
		c.dagRunsCurrentlyDesc,
		prometheus.GaugeValue,
		currentlyRunning,
	)

	for status, count := range statusCounts {
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsTotalDesc,
			prometheus.CounterValue,
			count,
			status,
		)
	}

	// Emit per-DAG metrics
	for dagName, dm := range perDAG {
		// Currently running per DAG
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsCurrentlyByDAGDesc,
			prometheus.GaugeValue,
			dm.running,
			dagName,
		)

		// Status counts per DAG
		for status, count := range dm.statusCounts {
			ch <- prometheus.MustNewConstMetric(
				c.dagRunsTotalByDAGDesc,
				prometheus.CounterValue,
				count,
				dagName,
				status,
			)
		}

		// Duration histogram per DAG
		emitHistogram(ch, c.dagRunDurationDesc, dm.durations, dagRunDurationBuckets, dagName)

		// Queue wait time histogram per DAG
		emitHistogram(ch, c.queueWaitTimeDesc, dm.queueWaits, queueWaitBuckets, dagName)
	}

	// Collect queue metrics
	c.collectQueueMetrics(ctx, ch)
}

func (c *Collector) collectDAGMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	// Get all DAGs using List with empty options to get all
	result, _, err := c.dagStore.List(ctx, exec.ListDAGsOptions{})
	if err != nil {
		return
	}

	totalDAGs := float64(result.TotalCount)

	ch <- prometheus.MustNewConstMetric(
		c.dagsTotalDesc,
		prometheus.GaugeValue,
		totalDAGs,
	)
}

func (c *Collector) collectQueueMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.queueStore == nil {
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsQueuedDesc,
			prometheus.GaugeValue,
			0,
		)
		return
	}

	items, err := c.queueStore.All(ctx)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsQueuedDesc,
			prometheus.GaugeValue,
			0,
		)
		return
	}

	// Emit aggregate queue count (backward compatible)
	ch <- prometheus.MustNewConstMetric(
		c.dagRunsQueuedDesc,
		prometheus.GaugeValue,
		float64(len(items)),
	)

	// Aggregate per-DAG queue counts
	perDAGQueue := make(map[string]float64)
	for _, item := range items {
		if item == nil {
			continue
		}
		data, err := item.Data()
		if err != nil || data == nil {
			continue
		}
		perDAGQueue[data.Name]++
	}

	// Emit per-DAG queue metrics
	for dagName, count := range perDAGQueue {
		ch <- prometheus.MustNewConstMetric(
			c.dagRunsQueuedByDAGDesc,
			prometheus.GaugeValue,
			count,
			dagName,
		)
	}
}

// isCompletedStatus returns true if the status represents a terminal state
func isCompletedStatus(s core.Status) bool {
	switch s {
	case core.Succeeded, core.Failed, core.Aborted, core.PartiallySucceeded, core.Rejected:
		return true
	case core.NotStarted, core.Running, core.Queued, core.Waiting:
		return false
	}
	return false
}

// calculateDuration computes the duration in seconds between two RFC3339 time strings
func calculateDuration(start, end string) float64 {
	if start == "" || end == "" {
		return 0
	}
	startTime, err := stringutil.ParseTime(start)
	if err != nil || startTime.IsZero() {
		return 0
	}
	endTime, err := stringutil.ParseTime(end)
	if err != nil || endTime.IsZero() {
		return 0
	}
	duration := endTime.Sub(startTime).Seconds()
	if duration < 0 {
		return 0
	}
	return duration
}

// emitHistogram creates and sends a histogram metric from observed values
func emitHistogram(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	observations []float64,
	buckets []float64,
	labelValues ...string,
) {
	if len(observations) == 0 {
		return
	}

	// Build bucket counts
	bucketCounts := make(map[float64]uint64)
	for _, bucket := range buckets {
		bucketCounts[bucket] = 0
	}

	var sum float64
	for _, obs := range observations {
		sum += obs
		for _, bucket := range buckets {
			if obs <= bucket {
				bucketCounts[bucket]++
			}
		}
	}

	ch <- prometheus.MustNewConstHistogram(
		desc,
		uint64(len(observations)),
		sum,
		bucketCounts,
		labelValues...,
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
