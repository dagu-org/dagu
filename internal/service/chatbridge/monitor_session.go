// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"time"
)

// Run starts the shared notification monitor loop.
func (m *NotificationMonitor) Run(ctx context.Context) {
	m.logger.Info("Notification monitor started")
	defer m.logger.Info("Notification monitor stopped")

	if m.lock == nil {
		m.initializeSession(ctx)
		m.runUnlockedLoop(ctx)
		return
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if !m.acquireNotificationLock(ctx) {
			return
		}

		sessionCtx, cancel := context.WithCancel(ctx)
		m.beginNotificationSession(cancel)
		m.initializeSession(sessionCtx)
		heartbeatDone := m.startNotificationLockHeartbeat(sessionCtx)

		continueWaiting := m.runOwnedLoop(ctx, sessionCtx)

		cancel()
		<-heartbeatDone
		m.releaseNotificationLock()
		m.endNotificationSession()
		if continueWaiting {
			m.resetInMemorySession()
		}
		if !continueWaiting {
			return
		}
	}
}

func (m *NotificationMonitor) runUnlockedLoop(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(m.cfg.SeenEvictInterval)
	defer evictTicker.Stop()

	for {
		if ready := m.takeReadyBatches(); len(ready) > 0 {
			inFlight := m.flushReadyBatches(ctx, ready)
			if ctx.Err() != nil {
				m.drainPendingBatches(ctx, inFlight)
				return
			}
			continue
		}

		select {
		case <-ctx.Done():
			m.drainPendingBatches(ctx, nil)
			return
		case <-ticker.C:
			m.syncPendingDestinations(ctx)
			m.pollSource(ctx)
		case <-evictTicker.C:
			m.evictStaleDelivered(ctx)
		case <-m.batcherReadyC():
		}
	}
}

func (m *NotificationMonitor) runOwnedLoop(parentCtx, sessionCtx context.Context) bool {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(m.cfg.SeenEvictInterval)
	defer evictTicker.Stop()

	for {
		if ready := m.takeReadyBatches(); len(ready) > 0 {
			inFlight := m.flushReadyBatches(sessionCtx, ready)
			if sessionCtx.Err() != nil {
				return m.finishOwnedLoop(parentCtx, inFlight)
			}
			continue
		}

		select {
		case <-parentCtx.Done():
			return m.finishOwnedLoop(parentCtx, nil)
		case <-sessionCtx.Done():
			return m.finishOwnedLoop(parentCtx, nil)
		case <-ticker.C:
			m.syncPendingDestinations(sessionCtx)
			m.pollSource(sessionCtx)
		case <-evictTicker.C:
			m.evictStaleDelivered(sessionCtx)
		case <-m.batcherReadyC():
		}
	}
}

func (m *NotificationMonitor) finishOwnedLoop(parentCtx context.Context, inFlight *NotificationPendingBatch) bool {
	if parentCtx.Err() != nil {
		if m.ownsNotificationLock() {
			m.drainPendingBatches(parentCtx, inFlight)
		} else {
			m.stopPendingBatches()
		}
		return false
	}

	m.stopPendingBatches()
	return true
}

func (m *NotificationMonitor) acquireNotificationLock(ctx context.Context) bool {
	if m.lock == nil {
		return true
	}

	for {
		if err := m.lock.Lock(ctx); err != nil {
			if ctx.Err() != nil {
				return false
			}
			m.logger.Warn("Failed to acquire notification lock",
				slog.String("lock_dir", m.lockDir),
				slog.String("error", err.Error()),
			)
			select {
			case <-ctx.Done():
				return false
			case <-time.After(DefaultNotificationLockRetryInterval):
			}
			continue
		}

		m.logger.Info("Acquired notification lock; notifications are active",
			slog.String("lock_dir", m.lockDir),
		)
		return true
	}
}

func (m *NotificationMonitor) releaseNotificationLock() {
	if m.lock == nil || !m.lock.IsHeldByMe() {
		return
	}
	if err := m.lock.Unlock(); err != nil {
		m.logger.Warn("Failed to release notification lock",
			slog.String("lock_dir", m.lockDir),
			slog.String("error", err.Error()),
		)
	}
}

func (m *NotificationMonitor) startNotificationLockHeartbeat(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if m.lock == nil {
		close(done)
		return done
	}

	go func() {
		defer close(done)

		ticker := time.NewTicker(DefaultNotificationLockHeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.lock.Heartbeat(ctx); err != nil {
					m.signalLockLoss("Notification lock heartbeat failed; entering standby", err)
					return
				}
			}
		}
	}()

	return done
}

func (m *NotificationMonitor) beginNotificationSession(cancel context.CancelFunc) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessionCancel = cancel
	m.sessionLost = false
}

func (m *NotificationMonitor) endNotificationSession() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessionCancel = nil
	m.sessionLost = false
}

func (m *NotificationMonitor) signalLockLoss(message string, err error) {
	if m.lock == nil {
		return
	}

	m.sessionMu.Lock()
	if m.sessionCancel == nil || m.sessionLost {
		m.sessionMu.Unlock()
		return
	}
	m.sessionLost = true
	cancel := m.sessionCancel
	m.sessionMu.Unlock()

	attrs := []any{slog.String("lock_dir", m.lockDir)}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	m.logger.Warn(message, attrs...)
	cancel()
}

func (m *NotificationMonitor) canPersistNotificationState(message string) bool {
	return m.ensureNotificationLockOwnership(message)
}

func (m *NotificationMonitor) canMutateNotificationState(message string) bool {
	return m.ensureNotificationLockOwnership(message)
}

func (m *NotificationMonitor) ensureNotificationLockOwnership(message string) bool {
	if m.lock == nil {
		return true
	}
	if m.lock.IsHeldByMe() {
		return true
	}

	m.sessionMu.Lock()
	sessionActive := m.sessionCancel != nil
	m.sessionMu.Unlock()
	if sessionActive {
		m.signalLockLoss(message, nil)
	}
	return false
}

func (m *NotificationMonitor) ownsNotificationLock() bool {
	if m.lock == nil {
		return true
	}
	return m.lock.IsHeldByMe()
}

func (m *NotificationMonitor) notificationSessionActive() bool {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	return m.sessionCancel != nil
}

func (m *NotificationMonitor) resetInMemorySession() {
	m.stateMu.Lock()
	m.state = newNotificationMonitorState()
	m.lastBootstrapFailure = ""
	m.stateMu.Unlock()
	m.resetBatcher()
}

func (m *NotificationMonitor) currentBatcher() *NotificationBatcher {
	m.batcherMu.RLock()
	defer m.batcherMu.RUnlock()
	return m.batcher
}

func (m *NotificationMonitor) batcherReadyC() <-chan struct{} {
	return m.currentBatcher().ReadyC()
}

func (m *NotificationMonitor) takeReadyBatches() []NotificationPendingBatch {
	return m.currentBatcher().TakeReady()
}

func (m *NotificationMonitor) enqueueBatch(destination string, event NotificationEvent) bool {
	return m.currentBatcher().Enqueue(destination, event)
}

func (m *NotificationMonitor) resetBatcher() {
	replacement := NewNotificationBatcher(m.cfg.UrgentWindow, m.cfg.SuccessWindow)

	m.batcherMu.Lock()
	current := m.batcher
	m.batcher = replacement
	m.batcherMu.Unlock()

	if current != nil {
		current.Stop()
	}
}

func (m *NotificationMonitor) drainAndResetBatcher() []NotificationPendingBatch {
	replacement := NewNotificationBatcher(m.cfg.UrgentWindow, m.cfg.SuccessWindow)

	m.batcherMu.Lock()
	current := m.batcher
	m.batcher = replacement
	m.batcherMu.Unlock()

	if current == nil {
		return nil
	}
	return current.DrainAndStop()
}

func (m *NotificationMonitor) discardPendingDestinations(destinations []string) {
	if len(destinations) == 0 {
		return
	}
	m.currentBatcher().DiscardDestinations(destinations)
}
