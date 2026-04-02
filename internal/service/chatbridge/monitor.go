// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/eventstore"
)

const (
	DefaultNotificationMonitorPollInterval   = 10 * time.Second
	DefaultNotificationSeenEvictInterval     = 10 * time.Minute
	DefaultNotificationSeenTTL               = 2 * time.Hour
	DefaultNotificationFlushTimeout          = 30 * time.Second
	DefaultNotificationLockHeartbeatInterval = time.Second
	DefaultNotificationLockRetryInterval     = 50 * time.Millisecond
	DefaultNotificationLockStaleThreshold    = 45 * time.Second
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
	lock         dirlock.DirLock
	lockDir      string
	transport    NotificationTransport
	logger       *slog.Logger
	cfg          NotificationMonitorConfig

	batcherMu sync.RWMutex
	batcher   *NotificationBatcher

	stateMu sync.Mutex
	state   notificationMonitorState

	sessionMu     sync.Mutex
	sessionCancel context.CancelFunc
	sessionLost   bool

	lastBootstrapFailure string
}

type queuedNotification struct {
	destination string
	event       NotificationEvent
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

	stateStore := newNotificationStateStore(stateFile)
	lockDir := notificationStateLockDir(stateFile)

	var lock dirlock.DirLock
	if lockDir != "" {
		lock = dirlock.New(lockDir, &dirlock.LockOptions{
			StaleThreshold: DefaultNotificationLockStaleThreshold,
			RetryInterval:  DefaultNotificationLockRetryInterval,
			OnWait: func() {
				logger.Info("Notification lock is held by another process; notifications are on standby",
					slog.String("lock_dir", lockDir),
				)
			},
		})
	}

	return &NotificationMonitor{
		eventService: eventService,
		stateStore:   stateStore,
		lock:         lock,
		lockDir:      lockDir,
		transport:    transport,
		logger:       logger,
		cfg:          cfg,
		batcher:      NewNotificationBatcher(cfg.UrgentWindow, cfg.SuccessWindow),
		state:        newNotificationMonitorState(),
	}
}

// NotifyCompletion queues a status update for every destination that has not yet acknowledged it.
func (m *NotificationMonitor) NotifyCompletion(status *exec.DAGRunStatus) bool {
	if status == nil {
		return false
	}
	if !m.canMutateNotificationState("Notification lock is not held; cannot queue notification") {
		return false
	}

	m.logger.Info("DAG run notification queued",
		slog.String("dag", status.Name),
		slog.String("status", status.Status.String()),
		slog.String("dag_run_id", status.DAGRunID),
	)
	eventType, _ := eventstore.PersistedDAGRunEventTypeForStatus(status.Status)
	snapshot := cloneNotificationStatus(status)

	event := NotificationEvent{
		Key:        NotificationSeenKey(status),
		Kind:       eventstore.KindDAGRun,
		Type:       eventType,
		DAGRun:     snapshot,
		ObservedAt: time.Now().UTC(),
	}
	return m.enqueueEvents(context.Background(), nil, []NotificationEvent{event})
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

func (m *NotificationMonitor) initializeSession(ctx context.Context) {
	m.loadPersistedState(ctx)

	destinations := m.transport.NotificationDestinations()
	var removed []string

	m.stateMu.Lock()
	persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		var changed bool
		removed, changed = reconcileDestinations(candidate, destinations)
		return changed
	})
	m.stateMu.Unlock()
	if !persisted {
		return
	}
	if len(removed) > 0 {
		m.discardPendingDestinations(removed)
	}

	m.ensureBootstrapped(ctx)
	m.requeuePending(destinations)
}

func (m *NotificationMonitor) loadPersistedState(ctx context.Context) {
	state := newNotificationMonitorState()
	if m.stateStore != nil {
		loadResult := m.stateStore.Load(ctx)
		state = loadResult.State
		if loadResult.Warning != nil {
			attrs := []any{slog.String("error", loadResult.Warning.Error())}
			if loadResult.QuarantinedPath != "" {
				attrs = append(attrs, slog.String("quarantined_path", loadResult.QuarantinedPath))
			}
			m.logger.Warn("Notification state was invalid; starting fresh", attrs...)
		}
	}

	m.stateMu.Lock()
	m.state = state
	m.state.normalize()
	m.lastBootstrapFailure = ""
	m.stateMu.Unlock()
}

func (m *NotificationMonitor) pollSource(ctx context.Context) {
	if m.eventService == nil {
		return
	}
	if !m.ensureNotificationLockOwnership("Notification lock lost before reading notification events") {
		return
	}
	if !m.ensureBootstrapped(ctx) {
		return
	}

	m.stateMu.Lock()
	if !m.state.Bootstrapped {
		m.stateMu.Unlock()
		return
	}
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
		notification, err := NotificationEventFromStoredEvent(event)
		if err != nil {
			m.logger.Warn("Failed to decode notification event payload",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		pending = append(pending, notification)
	}

	queued, committed := m.commitSourceProgress(ctx, destinations, nextCursor, pending)
	if !committed || len(queued) == 0 {
		return
	}
	for _, item := range queued {
		m.enqueueBatch(item.destination, item.event)
	}
}

func (m *NotificationMonitor) syncPendingDestinations(ctx context.Context) {
	if !m.ensureNotificationLockOwnership("Notification lock lost before syncing destinations") {
		return
	}

	destinations := m.transport.NotificationDestinations()
	var removed []string

	m.stateMu.Lock()
	persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		var changed bool
		removed, changed = reconcileDestinations(candidate, destinations)
		return changed
	})
	m.stateMu.Unlock()
	if !persisted {
		return
	}
	if len(removed) > 0 {
		m.discardPendingDestinations(removed)
	}

	m.requeuePending(destinations)
}

func (m *NotificationMonitor) enqueueEvents(ctx context.Context, destinations []string, events []NotificationEvent) bool {
	if len(events) == 0 {
		return false
	}
	if !m.canMutateNotificationState("Notification lock is not held; cannot enqueue notification events") {
		return false
	}
	if destinations == nil {
		destinations = m.transport.NotificationDestinations()
	}
	if len(destinations) == 0 {
		m.logger.Warn("No notification destinations configured, cannot send notification")
		return false
	}

	var queued []queuedNotification
	accepted := false

	m.stateMu.Lock()
	persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		changed := ensureDestinations(candidate, destinations)
		var enqueueChanged bool
		queued, enqueueChanged, accepted = enqueueNotifications(candidate, destinations, events)
		return changed || enqueueChanged
	})
	m.stateMu.Unlock()
	if !persisted {
		return false
	}

	for _, item := range queued {
		m.enqueueBatch(item.destination, item.event)
	}
	return accepted
}

func (m *NotificationMonitor) requeuePending(destinations []string) {
	if len(destinations) == 0 {
		return
	}
	if !m.ensureNotificationLockOwnership("Notification lock lost before requeueing pending notifications") {
		return
	}

	allowed := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		allowed[destination] = struct{}{}
	}

	var queued []queuedNotification

	m.stateMu.Lock()
	for destination, destState := range m.state.Destinations {
		if _, ok := allowed[destination]; !ok || destState == nil {
			continue
		}
		for _, event := range destState.Pending {
			if event.Key == "" {
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
		m.enqueueBatch(item.destination, item.event)
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
	if !m.ensureNotificationLockOwnership("Notification lock lost before delivering notification batch") {
		return false
	}

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
	drained := m.drainAndResetBatcher()
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

func (m *NotificationMonitor) stopPendingBatches() {
	m.resetBatcher()
}

func (m *NotificationMonitor) markBatchDelivered(ctx context.Context, destination string, batch NotificationBatch) {
	if !m.ensureNotificationLockOwnership("Notification lock lost before acknowledging delivered batch") {
		return
	}

	now := time.Now().UTC()

	m.stateMu.Lock()
	m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		destState := destinationState(candidate, destination)
		changed := false
		for _, event := range batch.Events {
			if event.Key == "" {
				continue
			}
			if _, ok := destState.Pending[event.Key]; ok {
				delete(destState.Pending, event.Key)
				changed = true
			}
			// Refresh the delivery timestamp on each successful acknowledgement so
			// TTL-based eviction retains recently retried deliveries.
			if deliveredAt, ok := destState.Delivered[event.Key]; !ok || !deliveredAt.Equal(now) {
				destState.Delivered[event.Key] = now
				changed = true
			}
		}
		return changed
	})
	m.stateMu.Unlock()
}

func (m *NotificationMonitor) evictStaleDelivered(ctx context.Context) {
	if !m.ensureNotificationLockOwnership("Notification lock lost before evicting delivered notifications") {
		return
	}

	cutoff := time.Now().Add(-m.cfg.SeenTTL)
	changed := false

	m.stateMu.Lock()
	m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		changed = false
		for _, destination := range candidate.Destinations {
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
		return changed
	})
	m.stateMu.Unlock()
}

func (m *NotificationMonitor) saveCandidateStateLocked(ctx context.Context, state notificationMonitorState) bool {
	if !m.canPersistNotificationState("Notification lock lost before persisting notification state") {
		return false
	}
	if m.stateStore == nil {
		return true
	}
	if err := m.stateStore.Save(ctx, state); err != nil {
		m.logger.Warn("Failed to persist notification state", slog.String("error", err.Error()))
		return false
	}
	return true
}

func (m *NotificationMonitor) applyStateTransitionLocked(ctx context.Context, mutate func(candidate *notificationMonitorState) bool) bool {
	candidate := cloneNotificationMonitorState(m.state)
	if !mutate(&candidate) {
		return true
	}
	if !m.saveCandidateStateLocked(ctx, candidate) {
		return false
	}
	m.state = candidate
	return true
}

func (m *NotificationMonitor) ensureBootstrapped(ctx context.Context) bool {
	if !m.canMutateNotificationState("Notification lock lost before bootstrapping notification cursor") {
		return false
	}

	m.stateMu.Lock()
	if m.state.Bootstrapped {
		m.lastBootstrapFailure = ""
		m.stateMu.Unlock()
		return true
	}
	m.stateMu.Unlock()

	if m.eventService == nil {
		m.stateMu.Lock()
		if m.state.Bootstrapped {
			m.lastBootstrapFailure = ""
			m.stateMu.Unlock()
			return true
		}
		persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
			candidate.Bootstrapped = true
			return true
		})
		if !persisted {
			m.recordBootstrapFailure("Failed to persist notification bootstrap state")
			m.stateMu.Unlock()
			return false
		}
		m.lastBootstrapFailure = ""
		m.stateMu.Unlock()
		return true
	}

	cursor, err := m.eventService.NotificationHeadCursor(ctx)
	if err != nil {
		m.recordBootstrapFailure("Failed to bootstrap notification cursor: " + err.Error())
		return false
	}

	m.stateMu.Lock()
	if m.state.Bootstrapped {
		m.lastBootstrapFailure = ""
		m.stateMu.Unlock()
		return true
	}
	persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		candidate.SourceCursor = cursor.Normalize()
		candidate.Bootstrapped = true
		return true
	})
	if !persisted {
		m.recordBootstrapFailure("Failed to persist notification bootstrap state")
		m.stateMu.Unlock()
		return false
	}
	m.lastBootstrapFailure = ""
	m.stateMu.Unlock()
	return true
}

func (m *NotificationMonitor) commitSourceProgress(ctx context.Context, destinations []string, nextCursor eventstore.NotificationCursor, events []NotificationEvent) ([]queuedNotification, bool) {
	if !m.canMutateNotificationState("Notification lock lost before advancing notification cursor") {
		return nil, false
	}

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if !m.state.Bootstrapped {
		return nil, false
	}

	var (
		queued   []queuedNotification
		accepted bool
	)
	persisted := m.applyStateTransitionLocked(ctx, func(candidate *notificationMonitorState) bool {
		changed := ensureDestinations(candidate, destinations)
		cursorChanged := !candidate.SourceCursor.Equal(nextCursor)
		candidate.SourceCursor = nextCursor.Normalize()
		var enqueueChanged bool
		queued, enqueueChanged, accepted = enqueueNotifications(candidate, destinations, events)
		return changed || cursorChanged || enqueueChanged
	})
	if !persisted {
		return nil, false
	}
	if !accepted {
		return nil, true
	}
	return queued, true
}

func (m *NotificationMonitor) recordBootstrapFailure(message string) {
	if message == m.lastBootstrapFailure {
		return
	}
	m.lastBootstrapFailure = message
	m.logger.Warn(message)
}

func notificationStateLockDir(stateFile string) string {
	if stateFile == "" {
		return ""
	}
	return filepath.Clean(stateFile) + ".lock"
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
