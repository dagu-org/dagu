// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
)

const (
	DefaultNotificationMonitorPollInterval = 10 * time.Second
	DefaultNotificationSeenEvictInterval   = 10 * time.Minute
	DefaultNotificationSeenTTL             = 2 * time.Hour
	DefaultNotificationFlushTimeout        = 30 * time.Second
)

// NotificationMonitorConfig controls polling, batching, and shutdown behavior.
type NotificationMonitorConfig struct {
	PollInterval      time.Duration
	SeenEvictInterval time.Duration
	SeenTTL           time.Duration
	FlushTimeout      time.Duration
	UrgentWindow      time.Duration
	SuccessWindow     time.Duration
}

// DefaultNotificationMonitorConfig returns the default monitor settings.
func DefaultNotificationMonitorConfig() NotificationMonitorConfig {
	return NotificationMonitorConfig{
		PollInterval:      DefaultNotificationMonitorPollInterval,
		SeenEvictInterval: DefaultNotificationSeenEvictInterval,
		SeenTTL:           DefaultNotificationSeenTTL,
		FlushTimeout:      DefaultNotificationFlushTimeout,
		UrgentWindow:      DefaultUrgentNotificationWindow,
		SuccessWindow:     DefaultSuccessNotificationWindow,
	}
}

// NotificationTransport supplies destination discovery and transport-specific delivery.
type NotificationTransport interface {
	NotificationDestinations() []string
	FlushNotificationBatch(ctx context.Context, destination string, batch NotificationBatch, allowLLM bool) bool
}

// NotificationMonitor owns polling, batching, delivered-state tracking, and shutdown drain.
type NotificationMonitor struct {
	dagRunStore exec.DAGRunStore
	transport   NotificationTransport
	logger      *slog.Logger
	cfg         NotificationMonitorConfig
	batcher     *NotificationBatcher
	delivered   sync.Map
}

// NewNotificationMonitor creates a shared notification monitor.
func NewNotificationMonitor(
	dagRunStore exec.DAGRunStore,
	transport NotificationTransport,
	logger *slog.Logger,
	cfg NotificationMonitorConfig,
) *NotificationMonitor {
	normalizeNotificationMonitorConfig(&cfg)
	return &NotificationMonitor{
		dagRunStore: dagRunStore,
		transport:   transport,
		logger:      logger,
		cfg:         cfg,
		batcher:     NewNotificationBatcher(cfg.UrgentWindow, cfg.SuccessWindow),
	}
}

// Run starts the shared notification monitor loop.
func (m *NotificationMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run monitor started")
	m.seedDelivered(ctx)

	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(m.cfg.SeenEvictInterval)
	defer evictTicker.Stop()

	for {
		if ready := m.batcher.TakeReady(); len(ready) > 0 {
			inFlight := m.flushReadyBatches(ctx, ready)
			if ctx.Err() != nil {
				m.drainPendingBatches(ctx, inFlight)
				m.logger.Info("DAG run monitor stopped")
				return
			}
			continue
		}

		select {
		case <-ctx.Done():
			m.drainPendingBatches(ctx, nil)
			m.logger.Info("DAG run monitor stopped")
			return
		case <-ticker.C:
			m.checkForCompletions(ctx)
		case <-evictTicker.C:
			m.evictStaleDelivered()
		case <-m.batcher.ReadyC():
		}
	}
}

// NotifyCompletion queues a status update for every destination that has not yet acknowledged it.
func (m *NotificationMonitor) NotifyCompletion(s *exec.DAGRunStatus) bool {
	if s == nil {
		return false
	}

	m.logger.Info("DAG run notification queued",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("dag_run_id", s.DAGRunID),
	)

	destinations := m.transport.NotificationDestinations()
	if len(destinations) == 0 {
		m.logger.Warn("No notification destinations configured, cannot send notification")
		return false
	}

	accepted := false
	for _, destination := range destinations {
		if m.IsDelivered(destination, s) {
			continue
		}
		if m.batcher.Enqueue(destination, s) {
			accepted = true
		}
	}
	return accepted
}

// IsDelivered reports whether a destination has already acknowledged a status.
func (m *NotificationMonitor) IsDelivered(destination string, s *exec.DAGRunStatus) bool {
	_, ok := m.delivered.Load(notificationDeliveredKey(destination, s))
	return ok
}

func (m *NotificationMonitor) seedDelivered(ctx context.Context) {
	if m.dagRunStore == nil {
		return
	}

	destinations := m.transport.NotificationDestinations()
	if len(destinations) == 0 {
		return
	}

	now := time.Now()
	from := exec.NewUTC(now.Add(-24 * time.Hour))
	to := exec.NewUTC(now)

	statuses, err := m.dagRunStore.ListStatuses(ctx,
		exec.WithFrom(from),
		exec.WithTo(to),
	)
	if err != nil {
		m.logger.Warn("Failed to seed monitor with existing runs", slog.String("error", err.Error()))
		return
	}

	for _, status := range statuses {
		if status.Status.IsActive() {
			continue
		}
		for _, destination := range destinations {
			m.markDelivered(destination, status)
		}
	}

	m.logger.Info("DAG run monitor seeded", slog.Int("existing_runs", len(statuses)))
}

func (m *NotificationMonitor) checkForCompletions(ctx context.Context) {
	if m.dagRunStore == nil {
		return
	}

	now := time.Now()
	from := exec.NewUTC(now.Add(-1 * time.Hour))
	to := exec.NewUTC(now)

	statuses, err := m.dagRunStore.ListStatuses(ctx,
		exec.WithFrom(from),
		exec.WithTo(to),
		exec.WithStatuses(NotificationStatuses),
	)
	if err != nil {
		m.logger.Debug("Failed to list DAG run statuses", slog.String("error", err.Error()))
		return
	}

	for _, status := range statuses {
		m.NotifyCompletion(status)
	}
}

func (m *NotificationMonitor) evictStaleDelivered() {
	cutoff := time.Now().Add(-m.cfg.SeenTTL)
	m.delivered.Range(func(key, value any) bool {
		if ts, ok := value.(time.Time); ok && ts.Before(cutoff) {
			m.delivered.Delete(key)
		}
		return true
	})
}

func (m *NotificationMonitor) flushReadyBatches(ctx context.Context, ready []NotificationPendingBatch) *NotificationPendingBatch {
	for _, pending := range ready {
		acked := m.flushPendingBatch(ctx, pending, true)
		if !acked && ctx.Err() != nil {
			pendingCopy := pending
			return &pendingCopy
		}
	}
	return nil
}

func (m *NotificationMonitor) flushPendingBatch(ctx context.Context, pending NotificationPendingBatch, allowLLM bool) bool {
	flushCtx := ctx
	cancel := func() {}
	if allowLLM {
		flushCtx, cancel = context.WithTimeout(ctx, m.cfg.FlushTimeout)
	}
	defer cancel()

	acked := m.transport.FlushNotificationBatch(flushCtx, pending.Destination, pending.Batch, allowLLM)
	if acked {
		m.markBatchDelivered(pending.Destination, pending.Batch)
	}
	return acked
}

func (m *NotificationMonitor) drainPendingBatches(ctx context.Context, inFlight *NotificationPendingBatch) {
	drained := m.batcher.DrainAndStop()
	if inFlight != nil {
		drained = append([]NotificationPendingBatch{*inFlight}, drained...)
	}
	if len(drained) == 0 {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.cfg.FlushTimeout)
	defer cancel()

	for _, pending := range drained {
		if shutdownCtx.Err() != nil {
			return
		}
		m.flushPendingBatch(shutdownCtx, pending, false)
	}
}

func (m *NotificationMonitor) markBatchDelivered(destination string, batch NotificationBatch) {
	for _, event := range batch.Events {
		m.markDelivered(destination, event.Status)
	}
}

func (m *NotificationMonitor) markDelivered(destination string, s *exec.DAGRunStatus) {
	m.delivered.Store(notificationDeliveredKey(destination, s), time.Now())
}

func notificationDeliveredKey(destination string, s *exec.DAGRunStatus) string {
	return destination + "|" + NotificationSeenKey(s)
}

func normalizeNotificationMonitorConfig(cfg *NotificationMonitorConfig) {
	defaults := DefaultNotificationMonitorConfig()
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaults.PollInterval
	}
	if cfg.SeenEvictInterval <= 0 {
		cfg.SeenEvictInterval = defaults.SeenEvictInterval
	}
	if cfg.SeenTTL <= 0 {
		cfg.SeenTTL = defaults.SeenTTL
	}
	if cfg.FlushTimeout <= 0 {
		cfg.FlushTimeout = defaults.FlushTimeout
	}
	if cfg.UrgentWindow <= 0 {
		cfg.UrgentWindow = defaults.UrgentWindow
	}
	if cfg.SuccessWindow <= 0 {
		cfg.SuccessWindow = defaults.SuccessWindow
	}
}
