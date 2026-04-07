// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/dirlock"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/eventstore"
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

var defaultInterestedNotificationEventTypes = []eventstore.EventType{
	eventstore.TypeDAGRunWaiting,
	eventstore.TypeDAGRunSucceeded,
	eventstore.TypeDAGRunFailed,
	eventstore.TypeDAGRunAborted,
	eventstore.TypeDAGRunRejected,
}

// NotificationMonitorConfig controls source polling, batching, and shutdown behavior.
type NotificationMonitorConfig struct {
	PollInterval         time.Duration
	SeenEvictInterval    time.Duration
	SeenTTL              time.Duration
	FlushTimeout         time.Duration
	UrgentWindow         time.Duration
	SuccessWindow        time.Duration
	InterestedEventTypes []eventstore.EventType
}

// DefaultNotificationMonitorConfig returns the default monitor settings.
func DefaultNotificationMonitorConfig() NotificationMonitorConfig {
	return NotificationMonitorConfig{
		PollInterval:         DefaultNotificationMonitorPollInterval,
		SeenEvictInterval:    DefaultNotificationSeenEvictInterval,
		SeenTTL:              DefaultNotificationSeenTTL,
		FlushTimeout:         DefaultNotificationFlushTimeout,
		UrgentWindow:         DefaultUrgentNotificationWindow,
		SuccessWindow:        DefaultSuccessNotificationWindow,
		InterestedEventTypes: append([]eventstore.EventType(nil), defaultInterestedNotificationEventTypes...),
	}
}

func interestedEventTypeSet(eventTypes []eventstore.EventType) map[eventstore.EventType]struct{} {
	set := make(map[eventstore.EventType]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		if eventType == "" {
			continue
		}
		set[eventType] = struct{}{}
	}
	return set
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
	interested   map[eventstore.EventType]struct{}

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
		interested:   interestedEventTypeSet(cfg.InterestedEventTypes),
		batcher:      NewNotificationBatcher(cfg.UrgentWindow, cfg.SuccessWindow),
		state:        newNotificationMonitorState(),
	}
}

// NotifyCompletion queues a status update for every destination that has not yet acknowledged it.
func (m *NotificationMonitor) NotifyCompletion(status *exec.DAGRunStatus) bool {
	if status == nil {
		return false
	}
	eventType, ok := eventstore.PersistedDAGRunEventTypeForStatus(status.Status)
	if !ok || !m.isInterestedEventType(eventType) {
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
	if cfg.InterestedEventTypes == nil {
		cfg.InterestedEventTypes = append([]eventstore.EventType(nil), defaults.InterestedEventTypes...)
	}
}

func (m *NotificationMonitor) isInterestedEventType(eventType eventstore.EventType) bool {
	if eventType == "" {
		return false
	}
	_, ok := m.interested[eventType]
	return ok
}
