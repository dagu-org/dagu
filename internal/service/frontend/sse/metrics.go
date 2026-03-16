// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for SSE operations.
type Metrics struct {
	messagesSent            *prometheus.CounterVec
	fetchErrors             *prometheus.CounterVec
	fetchDuration           *prometheus.HistogramVec
	multiplexSessionsActive prometheus.Gauge
	topicsPerSession        prometheus.Histogram
	topicMutations          *prometheus.CounterVec
	backpressureDisconnects prometheus.Counter
	unknownSessionMutations prometheus.Counter
}

// NewMetrics creates and registers SSE metrics with the given registry.
func NewMetrics(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
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
		multiplexSessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dagu_sse_multiplex_sessions_active",
			Help: "Current number of active multiplexed SSE sessions",
		}),
		topicsPerSession: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "dagu_sse_topics_per_session",
			Help:    "Histogram of topics subscribed per multiplexed SSE session",
			Buckets: []float64{1, 2, 4, 8, 12, 16, 20},
		}),
		topicMutations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dagu_sse_topic_mutations_total",
			Help: "Total number of multiplexed topic subscribe and unsubscribe operations",
		}, []string{"operation", "topic_type"}),
		backpressureDisconnects: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dagu_sse_backpressure_disconnects_total",
			Help: "Total number of multiplexed SSE sessions disconnected due to backpressure",
		}),
		unknownSessionMutations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dagu_sse_unknown_session_mutations_total",
			Help: "Total number of multiplexed topic mutation requests for unknown or expired sessions",
		}),
	}

	registry.MustRegister(
		m.messagesSent,
		m.fetchErrors,
		m.fetchDuration,
		m.multiplexSessionsActive,
		m.topicsPerSession,
		m.topicMutations,
		m.backpressureDisconnects,
		m.unknownSessionMutations,
	)

	return m
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

// MultiplexSessionConnected increments the active multiplex session count.
func (m *Metrics) MultiplexSessionConnected() {
	if m == nil {
		return
	}
	m.multiplexSessionsActive.Inc()
}

// MultiplexSessionDisconnected decrements the active multiplex session count.
func (m *Metrics) MultiplexSessionDisconnected() {
	if m == nil {
		return
	}
	m.multiplexSessionsActive.Dec()
}

// ObserveTopicsPerSession records the current subscription count for a multiplex session.
func (m *Metrics) ObserveTopicsPerSession(count int) {
	if m == nil {
		return
	}
	m.topicsPerSession.Observe(float64(count))
}

// TopicMutation increments the multiplex topic mutation counter.
func (m *Metrics) TopicMutation(operation, topicType string) {
	if m == nil {
		return
	}
	m.topicMutations.WithLabelValues(operation, topicType).Inc()
}

// BackpressureDisconnect increments the slow-client disconnect counter.
func (m *Metrics) BackpressureDisconnect() {
	if m == nil {
		return
	}
	m.backpressureDisconnects.Inc()
}

// UnknownSessionMutation increments the stale-session mutation counter.
func (m *Metrics) UnknownSessionMutation() {
	if m == nil {
		return
	}
	m.unknownSessionMutations.Inc()
}
