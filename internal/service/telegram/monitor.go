// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package telegram

import (
	"context"
	"log/slog"
	"strconv"
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

// seenTTL is how long a seen entry is kept before eviction. This must be
// longer than the lookback window in checkForCompletions (1 hour).
const seenTTL = 2 * time.Hour

const notificationFlushTimeout = 30 * time.Second

// DAGRunMonitor watches for DAG run completions and sends AI-generated
// notifications via Telegram.
type DAGRunMonitor struct {
	dagRunStore exec.DAGRunStore
	agentAPI    AgentService
	bot         *Bot
	logger      *slog.Logger

	// seen tracks DAG runs we've already notified about (dagRunID+attemptID -> true)
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

// Run starts the monitor loop. It polls for completed DAG runs and sends
// AI-generated notifications to all allowed Telegram chats.
func (m *DAGRunMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run monitor started")
	defer m.batcher.Stop()

	// Seed the seen set with currently completed runs so we don't notify
	// about old completions on startup.
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
	// Look at runs from the last hour to catch anything we might have missed
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

// seenKey returns a unique key for a DAG run status.
func seenKey(s *exec.DAGRunStatus) string {
	return chatbridge.NotificationSeenKey(s)
}

// markSeen marks a DAG run as already notified.
func (m *DAGRunMonitor) markSeen(s *exec.DAGRunStatus) {
	m.seen.Store(seenKey(s), time.Now())
}

// isSeen checks if we've already sent a notification for this run.
func (m *DAGRunMonitor) isSeen(s *exec.DAGRunStatus) bool {
	_, ok := m.seen.Load(seenKey(s))
	return ok
}

// evictStaleSeen removes entries from the seen map that are older than seenTTL.
func (m *DAGRunMonitor) evictStaleSeen() {
	cutoff := time.Now().Add(-seenTTL)
	m.seen.Range(func(key, value any) bool {
		if ts, ok := value.(time.Time); ok && ts.Before(cutoff) {
			m.seen.Delete(key)
		}
		return true
	})
}

// notifyCompletion queues a notification batch item for each chat.
func (m *DAGRunMonitor) notifyCompletion(_ context.Context, s *exec.DAGRunStatus) bool {
	m.logger.Info("DAG run notification queued",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("dag_run_id", s.DAGRunID),
	)

	if len(m.bot.allowedChats) == 0 {
		m.logger.Warn("No allowed chats configured, cannot send notification")
		return false
	}

	accepted := false
	for chatID := range m.bot.allowedChats {
		if m.batcher.Enqueue(strconv.FormatInt(chatID, 10), s) {
			accepted = true
		}
	}
	return accepted
}

func (m *DAGRunMonitor) flushBatch(chatIDKey string, batch chatbridge.NotificationBatch) {
	chatID, err := strconv.ParseInt(chatIDKey, 10, 64)
	if err != nil {
		m.logger.Warn("Failed to parse Telegram chat ID for notification batch",
			slog.String("chat_id", chatIDKey),
			slog.String("error", err.Error()),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), notificationFlushTimeout)
	defer cancel()

	m.flushChat(ctx, chatID, batch)
}

// flushChat appends the notification batch into the chat-scoped session.
func (m *DAGRunMonitor) flushChat(ctx context.Context, chatID int64, batch chatbridge.NotificationBatch) bool {
	user := m.bot.userIdentity(chatID)
	cs := m.bot.getOrCreateChat(chatID)
	sessionID := m.currentSessionID(cs)
	msg := m.buildNotificationMessage(ctx, sessionID, user, batch)
	sessionID, stored, ok := m.appendNotification(ctx, cs, sessionID, user, chatbridge.NotificationBatchDAGName(batch), msg)
	if !ok {
		return false
	}
	if m.bot.subscriptionActive(cs, sessionID) {
		return true
	}

	m.bot.sendLongText(chatID, msg.Content)
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
