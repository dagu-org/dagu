// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/eventstore"
)

const (
	DefaultNotificationMonitorPollInterval = 10 * time.Second
	DefaultNotificationSeenEvictInterval   = 10 * time.Minute
	DefaultNotificationSeenTTL             = 2 * time.Hour
	DefaultNotificationFlushTimeout        = 30 * time.Second
)

// NotificationMonitorConfig controls source polling, batching, and shutdown behavior.
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

// NotificationMonitor owns source polling, batching, durable delivery state, and shutdown drain.
type NotificationMonitor struct {
	eventService *eventstore.Service
	stateStore   *notificationStateStore
	transport    NotificationTransport
	logger       *slog.Logger
	cfg          NotificationMonitorConfig
	batcher      *NotificationBatcher

	stateMu sync.Mutex
	state   notificationMonitorState
}

// NewNotificationMonitor creates a shared notification monitor.
func NewNotificationMonitor(
	eventService *eventstore.Service,
	stateFile string,
	transport NotificationTransport,
	logger *slog.Logger,
	cfg NotificationMonitorConfig,
) *NotificationMonitor {
	normalizeNotificationMonitorConfig(&cfg)
	return &NotificationMonitor{
		eventService: eventService,
		stateStore:   newNotificationStateStore(stateFile),
		transport:    transport,
		logger:       logger,
		cfg:          cfg,
		batcher:      NewNotificationBatcher(cfg.UrgentWindow, cfg.SuccessWindow),
		state:        newNotificationMonitorState(),
	}
}

// Run starts the shared notification monitor loop.
func (m *NotificationMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run notification monitor started")
	m.initialize(ctx)

	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(m.cfg.SeenEvictInterval)
	defer evictTicker.Stop()

	for {
		if ready := m.batcher.TakeReady(); len(ready) > 0 {
			inFlight := m.flushReadyBatches(ctx, ready)
			if ctx.Err() != nil {
				m.drainPendingBatches(ctx, inFlight)
				m.logger.Info("DAG run notification monitor stopped")
				return
			}
			continue
		}

		select {
		case <-ctx.Done():
			m.drainPendingBatches(ctx, nil)
			m.logger.Info("DAG run notification monitor stopped")
			return
		case <-ticker.C:
			m.syncPendingDestinations(ctx)
			m.pollSource(ctx)
		case <-evictTicker.C:
			m.evictStaleDelivered(ctx)
		case <-m.batcher.ReadyC():
		}
	}
}

// NotifyCompletion queues a status update for every destination that has not yet acknowledged it.
func (m *NotificationMonitor) NotifyCompletion(status *exec.DAGRunStatus) bool {
	if status == nil {
		return false
	}

	m.logger.Info("DAG run notification queued",
		slog.String("dag", status.Name),
		slog.String("status", status.Status.String()),
		slog.String("dag_run_id", status.DAGRunID),
	)

	event := NotificationEvent{
		Key:        NotificationSeenKey(status),
		Status:     cloneNotificationStatus(status),
		ObservedAt: time.Now().UTC(),
	}
	return m.enqueueEvents(context.Background(), []string(nil), []NotificationEvent{event})
}

// IsDelivered reports whether a destination has already acknowledged a status.
func (m *NotificationMonitor) IsDelivered(destination string, status *exec.DAGRunStatus) bool {
	if destination == "" || status == nil {
		return false
	}
	key := NotificationSeenKey(status)

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	destState := m.state.Destinations[destination]
	if destState == nil {
		return false
	}
	_, ok := destState.Delivered[key]
	return ok
}

func (m *NotificationMonitor) initialize(ctx context.Context) {
	if m.stateStore != nil {
		state, err := m.stateStore.Load(ctx)
		if err != nil {
			m.logger.Warn("Failed to load notification state, starting fresh", slog.String("error", err.Error()))
		} else {
			m.state = state
		}
	}
	m.stateMu.Lock()
	m.state.normalize()

	destinations := m.transport.NotificationDestinations()
	changed := m.ensureDestinationsLocked(destinations)

	if !m.state.Bootstrapped {
		switch m.eventService {
		case nil:
			m.state.Bootstrapped = true
			changed = true
		default:
			cursor, err := m.eventService.NotificationHeadCursor(ctx)
			if err != nil {
				m.logger.Warn("Failed to bootstrap notification cursor", slog.String("error", err.Error()))
			} else {
				m.state.SourceCursor = cursor.Normalize()
				m.state.Bootstrapped = true
				changed = true
			}
		}
	}

	if changed {
		m.saveStateLocked(ctx)
	}
	m.stateMu.Unlock()
	m.requeuePending(destinations)
}

func (m *NotificationMonitor) pollSource(ctx context.Context) {
	if m.eventService == nil {
		return
	}

	m.stateMu.Lock()
	cursor := m.state.SourceCursor
	m.stateMu.Unlock()

	events, nextCursor, err := m.eventService.ReadNotificationEvents(ctx, cursor)
	if err != nil {
		m.logger.Debug("Failed to read notification events", slog.String("error", err.Error()))
		return
	}

	destinations := m.transport.NotificationDestinations()
	pending := make([]NotificationEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		status, err := eventstore.NotificationStatusFromEvent(event)
		if err != nil {
			m.logger.Warn("Failed to decode notification event payload",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		observedAt := event.RecordedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		pending = append(pending, NotificationEvent{
			Key:        NotificationSeenKey(status),
			Status:     status,
			ObservedAt: observedAt.UTC(),
		})
	}

	m.stateMu.Lock()
	m.ensureDestinationsLocked(destinations)
	cursorChanged := !m.state.SourceCursor.Equal(nextCursor)
	m.state.SourceCursor = nextCursor.Normalize()
	if cursorChanged {
		m.saveStateLocked(ctx)
	}
	m.stateMu.Unlock()

	if len(pending) == 0 {
		return
	}
	m.enqueueEvents(ctx, destinations, pending)
}

func (m *NotificationMonitor) syncPendingDestinations(ctx context.Context) {
	destinations := m.transport.NotificationDestinations()

	m.stateMu.Lock()
	changed := m.ensureDestinationsLocked(destinations)
	if changed {
		m.saveStateLocked(ctx)
	}
	m.stateMu.Unlock()

	m.requeuePending(destinations)
}

func (m *NotificationMonitor) enqueueEvents(ctx context.Context, destinations []string, events []NotificationEvent) bool {
	if len(events) == 0 {
		return false
	}
	if destinations == nil {
		destinations = m.transport.NotificationDestinations()
	}
	if len(destinations) == 0 {
		m.logger.Warn("No notification destinations configured, cannot send notification")
		return false
	}

	type queuedNotification struct {
		destination string
		event       NotificationEvent
	}

	var queued []queuedNotification
	accepted := false

	m.stateMu.Lock()
	changed := m.ensureDestinationsLocked(destinations)
	for _, destination := range destinations {
		destState := m.destinationStateLocked(destination)
		for _, event := range events {
			if event.Status == nil || event.Key == "" {
				continue
			}
			if _, ok := destState.Delivered[event.Key]; ok {
				continue
			}
			if pending, ok := destState.Pending[event.Key]; ok {
				queued = append(queued, queuedNotification{
					destination: destination,
					event:       pending,
				})
				accepted = true
				continue
			}

			runKey := NotificationRunKey(event.Status)
			for pendingKey, pending := range destState.Pending {
				if pending.Status == nil {
					continue
				}
				if NotificationRunKey(pending.Status) != runKey || pendingKey == event.Key {
					continue
				}
				delete(destState.Pending, pendingKey)
				changed = true
			}

			destState.Pending[event.Key] = NotificationEvent{
				Key:        event.Key,
				Status:     cloneNotificationStatus(event.Status),
				ObservedAt: event.ObservedAt,
			}
			queued = append(queued, queuedNotification{
				destination: destination,
				event:       event,
			})
			accepted = true
			changed = true
		}
	}
	if changed {
		m.saveStateLocked(ctx)
	}
	m.stateMu.Unlock()

	for _, item := range queued {
		m.batcher.Enqueue(item.destination, item.event)
	}
	return accepted
}

func (m *NotificationMonitor) requeuePending(destinations []string) {
	if len(destinations) == 0 {
		return
	}
	allowed := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		allowed[destination] = struct{}{}
	}

	type queuedNotification struct {
		destination string
		event       NotificationEvent
	}
	var queued []queuedNotification

	m.stateMu.Lock()
	for destination, destState := range m.state.Destinations {
		if _, ok := allowed[destination]; !ok || destState == nil {
			continue
		}
		for _, event := range destState.Pending {
			if event.Status == nil || event.Key == "" {
				continue
			}
			queued = append(queued, queuedNotification{
				destination: destination,
				event:       event,
			})
		}
	}
	m.stateMu.Unlock()

	slices.SortFunc(queued, func(a, b queuedNotification) int {
		if !a.event.ObservedAt.Equal(b.event.ObservedAt) {
			if a.event.ObservedAt.Before(b.event.ObservedAt) {
				return -1
			}
			return 1
		}
		switch {
		case a.destination < b.destination:
			return -1
		case a.destination > b.destination:
			return 1
		case a.event.Key < b.event.Key:
			return -1
		case a.event.Key > b.event.Key:
			return 1
		default:
			return 0
		}
	})

	for _, item := range queued {
		m.batcher.Enqueue(item.destination, item.event)
	}
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
	destinations := m.transport.NotificationDestinations()
	if !slices.Contains(destinations, pending.Destination) {
		return false
	}

	flushCtx := ctx
	cancel := func() {}
	if allowLLM {
		flushCtx, cancel = context.WithTimeout(ctx, m.cfg.FlushTimeout)
	}
	defer cancel()

	acked := m.transport.FlushNotificationBatch(flushCtx, pending.Destination, pending.Batch, allowLLM)
	if acked {
		m.markBatchDelivered(ctx, pending.Destination, pending.Batch)
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

func (m *NotificationMonitor) markBatchDelivered(ctx context.Context, destination string, batch NotificationBatch) {
	now := time.Now().UTC()

	m.stateMu.Lock()
	destState := m.destinationStateLocked(destination)
	for _, event := range batch.Events {
		if event.Key == "" {
			continue
		}
		delete(destState.Pending, event.Key)
		destState.Delivered[event.Key] = now
	}
	m.saveStateLocked(ctx)
	m.stateMu.Unlock()
}

func (m *NotificationMonitor) evictStaleDelivered(ctx context.Context) {
	cutoff := time.Now().Add(-m.cfg.SeenTTL)
	changed := false

	m.stateMu.Lock()
	for _, destination := range m.state.Destinations {
		if destination == nil {
			continue
		}
		for key, deliveredAt := range destination.Delivered {
			if deliveredAt.Before(cutoff) {
				delete(destination.Delivered, key)
				changed = true
			}
		}
	}
	if changed {
		m.saveStateLocked(ctx)
	}
	m.stateMu.Unlock()
}

func (m *NotificationMonitor) ensureDestinationsLocked(destinations []string) bool {
	changed := false
	for _, destination := range destinations {
		if destination == "" {
			continue
		}
		if _, ok := m.state.Destinations[destination]; ok {
			continue
		}
		m.state.Destinations[destination] = &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
		changed = true
	}
	return changed
}

func (m *NotificationMonitor) destinationStateLocked(destination string) *notificationDestinationState {
	if destination == "" {
		return &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
	}
	state, ok := m.state.Destinations[destination]
	if !ok || state == nil {
		state = &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
		m.state.Destinations[destination] = state
	}
	if state.Pending == nil {
		state.Pending = make(map[string]NotificationEvent)
	}
	if state.Delivered == nil {
		state.Delivered = make(map[string]time.Time)
	}
	return state
}

func (m *NotificationMonitor) saveStateLocked(ctx context.Context) {
	if m.stateStore == nil {
		return
	}
	if err := m.stateStore.Save(ctx, m.state); err != nil {
		m.logger.Warn("Failed to persist notification state", slog.String("error", err.Error()))
	}
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
