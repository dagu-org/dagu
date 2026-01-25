package sse

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for SSE operations.
type Metrics struct {
	clientsConnected prometheus.Gauge
	watchersActive   prometheus.Gauge
	messagesSent     *prometheus.CounterVec
	fetchErrors      *prometheus.CounterVec
	fetchDuration    *prometheus.HistogramVec
}

// NewMetrics creates and registers SSE metrics with the given registry.
func NewMetrics(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		clientsConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dagu_sse_clients_connected",
			Help: "Current number of connected SSE clients",
		}),
		watchersActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dagu_sse_watchers_active",
			Help: "Current number of active SSE watchers",
		}),
		messagesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dagu_sse_messages_sent_total",
			Help: "Total number of SSE messages sent by type",
		}, []string{"type"}),
		fetchErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dagu_sse_fetch_errors_total",
			Help: "Total number of SSE fetch errors by topic type",
		}, []string{"topic_type"}),
		fetchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dagu_sse_fetch_duration_seconds",
			Help:    "Duration of SSE data fetches by topic type",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		}, []string{"topic_type"}),
	}

	registry.MustRegister(
		m.clientsConnected,
		m.watchersActive,
		m.messagesSent,
		m.fetchErrors,
		m.fetchDuration,
	)

	return m
}

// ClientConnected increments the connected clients count.
func (m *Metrics) ClientConnected() {
	if m == nil {
		return
	}
	m.clientsConnected.Inc()
}

// ClientDisconnected decrements the connected clients count.
func (m *Metrics) ClientDisconnected() {
	if m == nil {
		return
	}
	m.clientsConnected.Dec()
}

// WatcherStarted increments the active watchers count.
func (m *Metrics) WatcherStarted() {
	if m == nil {
		return
	}
	m.watchersActive.Inc()
}

// WatcherStopped decrements the active watchers count.
func (m *Metrics) WatcherStopped() {
	if m == nil {
		return
	}
	m.watchersActive.Dec()
}

// MessageSent increments the messages sent counter for the given type.
func (m *Metrics) MessageSent(eventType string) {
	if m == nil {
		return
	}
	m.messagesSent.WithLabelValues(eventType).Inc()
}

// FetchError increments the fetch errors counter for the given topic type.
func (m *Metrics) FetchError(topicType string) {
	if m == nil {
		return
	}
	m.fetchErrors.WithLabelValues(topicType).Inc()
}

// RecordFetchDuration records the duration of a fetch operation for the given topic type.
func (m *Metrics) RecordFetchDuration(topicType string, duration time.Duration) {
	if m == nil {
		return
	}
	m.fetchDuration.WithLabelValues(topicType).Observe(duration.Seconds())
}
