// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"slices"
	"time"
)

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
