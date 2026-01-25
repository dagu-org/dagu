package sse

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()

	m := NewMetrics(registry)

	require.NotNil(t, m)
	assert.NotNil(t, m.clientsConnected)
	assert.NotNil(t, m.watchersActive)
	assert.NotNil(t, m.messagesSent)
	assert.NotNil(t, m.fetchErrors)

	// Verify metrics are registered by gathering them
	families, err := registry.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, family := range families {
		metricNames[family.GetName()] = true
	}

	// Initially gauges are 0 so they may not appear, but we can verify by incrementing
	m.ClientConnected()
	m.WatcherStarted()

	families, err = registry.Gather()
	require.NoError(t, err)

	metricNames = make(map[string]bool)
	for _, family := range families {
		metricNames[family.GetName()] = true
	}

	assert.True(t, metricNames["dagu_sse_clients_connected"])
	assert.True(t, metricNames["dagu_sse_watchers_active"])
}

func TestMetricsClientConnectedDisconnected(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewMetrics(registry)

	// Initial value should be 0
	assert.Equal(t, float64(0), getGaugeValue(t, m.clientsConnected))

	// Connect a client
	m.ClientConnected()
	assert.Equal(t, float64(1), getGaugeValue(t, m.clientsConnected))

	// Connect another client
	m.ClientConnected()
	assert.Equal(t, float64(2), getGaugeValue(t, m.clientsConnected))

	// Disconnect a client
	m.ClientDisconnected()
	assert.Equal(t, float64(1), getGaugeValue(t, m.clientsConnected))

	// Disconnect the last client
	m.ClientDisconnected()
	assert.Equal(t, float64(0), getGaugeValue(t, m.clientsConnected))
}

func TestMetricsWatcherStartedStopped(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewMetrics(registry)

	// Initial value should be 0
	assert.Equal(t, float64(0), getGaugeValue(t, m.watchersActive))

	// Start a watcher
	m.WatcherStarted()
	assert.Equal(t, float64(1), getGaugeValue(t, m.watchersActive))

	// Start another watcher
	m.WatcherStarted()
	assert.Equal(t, float64(2), getGaugeValue(t, m.watchersActive))

	// Stop a watcher
	m.WatcherStopped()
	assert.Equal(t, float64(1), getGaugeValue(t, m.watchersActive))

	// Stop the last watcher
	m.WatcherStopped()
	assert.Equal(t, float64(0), getGaugeValue(t, m.watchersActive))
}

func TestMetricsMessageSent(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewMetrics(registry)

	// Send messages of different types
	m.MessageSent(EventTypeData)
	m.MessageSent(EventTypeData)
	m.MessageSent(EventTypeHeartbeat)

	// Verify counts by type
	assert.Equal(t, float64(2), getCounterValue(t, m.messagesSent, EventTypeData))
	assert.Equal(t, float64(1), getCounterValue(t, m.messagesSent, EventTypeHeartbeat))
}

func TestMetricsFetchError(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewMetrics(registry)

	// Record fetch errors for different topic types
	m.FetchError(string(TopicTypeDAGRun))
	m.FetchError(string(TopicTypeDAGRun))
	m.FetchError(string(TopicTypeDAG))

	// Verify counts by topic type
	assert.Equal(t, float64(2), getFetchErrorValue(t, m.fetchErrors, string(TopicTypeDAGRun)))
	assert.Equal(t, float64(1), getFetchErrorValue(t, m.fetchErrors, string(TopicTypeDAG)))
}

func TestMetricsNilSafety(t *testing.T) {
	var m *Metrics

	// None of these should panic
	assert.NotPanics(t, func() { m.ClientConnected() })
	assert.NotPanics(t, func() { m.ClientDisconnected() })
	assert.NotPanics(t, func() { m.WatcherStarted() })
	assert.NotPanics(t, func() { m.WatcherStopped() })
	assert.NotPanics(t, func() { m.MessageSent(EventTypeData) })
	assert.NotPanics(t, func() { m.FetchError(string(TopicTypeDAGRun)) })
}

// Helper functions

func getGaugeValue(t *testing.T, gauge prometheus.Gauge) float64 {
	t.Helper()
	var metric dto.Metric
	err := gauge.Write(&metric)
	require.NoError(t, err)
	return metric.GetGauge().GetValue()
}

func getCounterValue(t *testing.T, counter *prometheus.CounterVec, eventType string) float64 {
	t.Helper()
	c, err := counter.GetMetricWithLabelValues(eventType)
	require.NoError(t, err)
	var metric dto.Metric
	err = c.Write(&metric)
	require.NoError(t, err)
	return metric.GetCounter().GetValue()
}

func getFetchErrorValue(t *testing.T, counter *prometheus.CounterVec, topicType string) float64 {
	t.Helper()
	c, err := counter.GetMetricWithLabelValues(topicType)
	require.NoError(t, err)
	var metric dto.Metric
	err = c.Write(&metric)
	require.NoError(t, err)
	return metric.GetCounter().GetValue()
}
