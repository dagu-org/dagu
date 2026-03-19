// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package slack

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/chatbridge"
)

// monitorPollInterval is how often the monitor checks for DAG run status changes.
const monitorPollInterval = 10 * time.Second

// seenEvictInterval is how often stale entries are purged from the seen map.
const seenEvictInterval = 10 * time.Minute

// seenTTL is how long a seen entry is kept before eviction.
const seenTTL = 2 * time.Hour

const notificationFlushTimeout = 30 * time.Second

// DAGRunMonitor watches for DAG run completions and sends AI-generated
// notifications via Slack.
type DAGRunMonitor struct {
	dagRunStore exec.DAGRunStore
	agentAPI    AgentService
	bot         *Bot
	logger      *slog.Logger

	seen    sync.Map
	batcher *chatbridge.NotificationBatcher
}

// NewDAGRunMonitor creates a new monitor instance.
func NewDAGRunMonitor(dagRunStore exec.DAGRunStore, agentAPI AgentService, bot *Bot, logger *slog.Logger) *DAGRunMonitor {
	monitor := &DAGRunMonitor{
		dagRunStore: dagRunStore,
		agentAPI:    agentAPI,
		bot:         bot,
		logger:      logger,
	}
	monitor.batcher = chatbridge.NewNotificationBatcher(
		chatbridge.DefaultUrgentNotificationWindow,
		chatbridge.DefaultSuccessNotificationWindow,
		monitor.flushBatch,
	)
	return monitor
}

// Run starts the monitor loop.
func (m *DAGRunMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run monitor started")
	defer m.batcher.Stop()

	m.seedSeen(ctx)

	ticker := time.NewTicker(monitorPollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(seenEvictInterval)
	defer evictTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("DAG run monitor stopped")
			return
		case <-ticker.C:
			m.checkForCompletions(ctx)
		case <-evictTicker.C:
			m.evictStaleSeen()
		}
	}
}

// seedSeen marks all currently completed runs as already seen.
func (m *DAGRunMonitor) seedSeen(ctx context.Context) {
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

	for _, s := range statuses {
		if !s.Status.IsActive() {
			m.markSeen(s)
		}
	}

	m.logger.Info("DAG run monitor seeded", slog.Int("existing_runs", len(statuses)))
}

// checkForCompletions polls for recently completed DAG runs and notifies.
func (m *DAGRunMonitor) checkForCompletions(ctx context.Context) {
	now := time.Now()
	from := exec.NewUTC(now.Add(-1 * time.Hour))
	to := exec.NewUTC(now)

	statuses, err := m.dagRunStore.ListStatuses(ctx,
		exec.WithFrom(from),
		exec.WithTo(to),
		exec.WithStatuses(chatbridge.NotificationStatuses),
	)
	if err != nil {
		m.logger.Debug("Failed to list DAG run statuses", slog.String("error", err.Error()))
		return
	}

	for _, s := range statuses {
		if m.isSeen(s) {
			continue
		}
		if m.notifyCompletion(ctx, s) {
			m.markSeen(s)
		}
	}
}

func seenKey(s *exec.DAGRunStatus) string {
	return chatbridge.NotificationSeenKey(s)
}

func (m *DAGRunMonitor) markSeen(s *exec.DAGRunStatus) {
	m.seen.Store(seenKey(s), time.Now())
}

func (m *DAGRunMonitor) isSeen(s *exec.DAGRunStatus) bool {
	_, ok := m.seen.Load(seenKey(s))
	return ok
}

func (m *DAGRunMonitor) evictStaleSeen() {
	cutoff := time.Now().Add(-seenTTL)
	m.seen.Range(func(key, value any) bool {
		if ts, ok := value.(time.Time); ok && ts.Before(cutoff) {
			m.seen.Delete(key)
		}
		return true
	})
}

// notifyCompletion queues a notification batch item for each destination.
func (m *DAGRunMonitor) notifyCompletion(_ context.Context, s *exec.DAGRunStatus) bool {
	m.logger.Info("DAG run notification queued",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("dag_run_id", s.DAGRunID),
	)

	if len(m.bot.allowedChannels) == 0 {
		m.logger.Warn("No allowed channels configured, cannot send notification")
		return false
	}

	accepted := false
	for channelID := range m.bot.allowedChannels {
		if m.batcher.Enqueue(channelID, s) {
			accepted = true
		}
	}
	return accepted
}

func (m *DAGRunMonitor) flushBatch(channelID string, batch chatbridge.NotificationBatch) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationFlushTimeout)
	defer cancel()

	if strings.HasPrefix(channelID, "D") {
		m.flushDirectMessage(ctx, channelID, batch)
		return
	}
	m.flushChannelThread(ctx, channelID, batch)
}

func (m *DAGRunMonitor) flushDirectMessage(ctx context.Context, channelID string, batch chatbridge.NotificationBatch) bool {
	convKey := channelID
	user := m.bot.userIdentity(convKey)
	cs := m.bot.getOrCreateChat(convKey, channelID, "")
	sessionID := m.currentSessionID(cs)

	msg := m.buildNotificationMessage(ctx, sessionID, user, batch)
	sessionID, stored, ok := m.appendNotification(ctx, cs, sessionID, user, chatbridge.NotificationBatchDAGName(batch), msg)
	if !ok {
		return false
	}
	if m.bot.subscriptionActive(cs, sessionID) {
		return true
	}

	m.bot.sendLongText(channelID, msg.Content)
	m.bot.markDelivered(cs, stored.SequenceID)
	return true
}

func (m *DAGRunMonitor) flushChannelThread(ctx context.Context, channelID string, batch chatbridge.NotificationBatch) bool {
	msg := m.buildNotificationMessage(ctx, "", m.bot.userIdentity(channelID), batch)
	threadTS := m.bot.sendLongRootThread(channelID, msg.Content)
	if threadTS == "" {
		return false
	}

	threadKey := channelID + ":" + threadTS
	m.bot.activeThreads.Store(threadKey, true)

	cs := m.bot.getOrCreateChat(threadKey, channelID, threadTS)
	user := m.bot.userIdentity(threadKey)

	sessionID, stored, err := chatbridge.AppendNotification(ctx, m.agentAPI, &cs.State, user, chatbridge.NotificationBatchDAGName(batch), m.bot.cfg.SafeMode, msg)
	if err != nil {
		m.logger.Warn("Failed to append threaded notification message",
			slog.String("session", sessionID),
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()),
		)
		return true
	}
	m.bot.markDelivered(cs, stored.SequenceID)
	return true
}

func (m *DAGRunMonitor) currentSessionID(cs *chatState) string {
	return cs.SessionID()
}

func (m *DAGRunMonitor) appendNotification(ctx context.Context, cs *chatState, sessionID string, user agent.UserIdentity, dagName string, msg agent.Message) (string, agent.Message, bool) {
	newSessionID, stored, err := chatbridge.AppendNotification(ctx, m.agentAPI, &cs.State, user, dagName, m.bot.cfg.SafeMode, msg)
	if err != nil {
		m.logger.Warn("Failed to append notification message",
			slog.String("session", sessionID),
			slog.String("error", err.Error()),
		)
		return "", agent.Message{}, false
	}
	return newSessionID, stored, true
}

func (m *DAGRunMonitor) buildNotificationMessage(ctx context.Context, sessionID string, user agent.UserIdentity, batch chatbridge.NotificationBatch) agent.Message {
	msg, err := chatbridge.GenerateNotificationMessage(ctx, m.agentAPI, sessionID, user, batch)
	if err != nil && len(batch.Events) > 0 && batch.Events[0].Status != nil {
		m.logger.Warn("Failed to generate AI notification, using deterministic fallback",
			slog.String("dag", batch.Events[0].Status.Name),
			slog.String("status", batch.Events[0].Status.Status.String()),
			slog.String("error", err.Error()),
		)
	}
	return msg
}
