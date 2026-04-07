// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"time"

	"github.com/dagucloud/dagu/internal/service/eventstore"
)

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

	events, nextCursor, err := m.eventService.ReadDAGRunEvents(ctx, cursor)
	if err != nil {
		m.logger.Debug("Failed to read DAG-run events", slog.String("error", err.Error()))
		return
	}

	destinations := m.transport.NotificationDestinations()
	pending := make([]NotificationEvent, 0, len(events))
	for _, event := range events {
		if event == nil || !m.isInterestedEventType(event.Type) {
			continue
		}
		status, err := eventstore.DAGRunStatusFromEvent(event)
		if err != nil {
			m.logger.Warn("Failed to decode DAG-run event payload",
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
			Kind:       eventstore.KindDAGRun,
			Type:       event.Type,
			DAGRun:     status,
			ObservedAt: observedAt.UTC(),
		})
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

	cursor, err := m.eventService.DAGRunHeadCursor(ctx)
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
