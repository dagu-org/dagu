// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()

	m := NewMetrics(registry)

	require.NotNil(t, m)
	assert.NotNil(t, m.messagesSent)
	assert.NotNil(t, m.fetchErrors)

	// Verify metrics are registered by gathering them after incrementing
	m.MessageSent(EventTypeData)

	families, err := registry.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, family := range families {
		metricNames[family.GetName()] = true
	}

	assert.True(t, metricNames["dagu_sse_messages_sent_total"])
}

func TestMetricsMessageSent(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	var m *Metrics

	// None of these should panic
	assert.NotPanics(t, func() { m.MessageSent(EventTypeData) })
	assert.NotPanics(t, func() { m.FetchError(string(TopicTypeDAGRun)) })
	assert.NotPanics(t, func() { m.MultiplexSessionConnected() })
	assert.NotPanics(t, func() { m.MultiplexSessionDisconnected() })
	assert.NotPanics(t, func() { m.ObserveTopicsPerSession(5) })
	assert.NotPanics(t, func() { m.TopicMutation("subscribe", "dag") })
	assert.NotPanics(t, func() { m.BackpressureDisconnect() })
	assert.NotPanics(t, func() { m.UnknownSessionMutation() })
}

// Helper functions

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
