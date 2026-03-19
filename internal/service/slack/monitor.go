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

	ctxMu   sync.RWMutex
	runCtx  context.Context
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
		runCtx:      context.Background(),
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
	m.setRunContext(ctx)

	m.seedSeen(ctx)

	ticker := time.NewTicker(monitorPollInterval)
	defer ticker.Stop()

	evictTicker := time.NewTicker(seenEvictInterval)
	defer evictTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.drainPendingBatches(ctx)
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
	destinations := m.destinations()
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

	for _, s := range statuses {
		if !s.Status.IsActive() {
			for _, destination := range destinations {
				m.markSeen(destination, s)
			}
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
		m.notifyCompletion(ctx, s)
	}
}

func seenKey(destination string, s *exec.DAGRunStatus) string {
	return destination + "|" + chatbridge.NotificationSeenKey(s)
}

func (m *DAGRunMonitor) markSeen(destination string, s *exec.DAGRunStatus) {
	m.seen.Store(seenKey(destination, s), time.Now())
}

func (m *DAGRunMonitor) isSeen(destination string, s *exec.DAGRunStatus) bool {
	_, ok := m.seen.Load(seenKey(destination, s))
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
	for _, channelID := range m.destinations() {
		if m.isSeen(channelID, s) {
			continue
		}
		if m.batcher.Enqueue(channelID, s) {
			accepted = true
		}
	}
	return accepted
}

func (m *DAGRunMonitor) flushBatch(channelID string, batch chatbridge.NotificationBatch) {
	ctx, cancel := context.WithTimeout(m.runContext(), notificationFlushTimeout)
	defer cancel()

	m.flushBatchWithPolicy(ctx, channelID, batch, true)
}

func (m *DAGRunMonitor) flushBatchWithPolicy(ctx context.Context, channelID string, batch chatbridge.NotificationBatch, allowLLM bool) bool {
	if strings.HasPrefix(channelID, "D") {
		return m.flushDirectMessage(ctx, channelID, batch, allowLLM)
	}
	return m.flushChannelThread(ctx, channelID, batch, allowLLM)
}

func (m *DAGRunMonitor) flushDirectMessage(ctx context.Context, channelID string, batch chatbridge.NotificationBatch, allowLLM bool) bool {
	convKey := channelID
	user := m.bot.userIdentity(convKey)
	cs := m.bot.getOrCreateChat(convKey, channelID, "")
	sessionID := m.currentSessionID(cs)

	msg := m.buildNotificationMessage(ctx, sessionID, user, batch, allowLLM)
	sessionID, stored, ok := m.appendNotification(ctx, cs, sessionID, user, chatbridge.NotificationBatchDAGName(batch), msg)
	if !ok {
		return false
	}
	m.markBatchSeen(channelID, batch)
	if m.bot.subscriptionActive(cs, sessionID) {
		return true
	}

	m.bot.sendLongText(channelID, msg.Content)
	m.bot.markDelivered(cs, stored.SequenceID)
	return true
}

func (m *DAGRunMonitor) flushChannelThread(ctx context.Context, channelID string, batch chatbridge.NotificationBatch, allowLLM bool) bool {
	msg := m.buildNotificationMessage(ctx, "", m.bot.userIdentity(channelID), batch, allowLLM)
	threadTS := m.bot.sendLongRootThread(channelID, msg.Content)
	if threadTS == "" {
		return false
	}
	m.markBatchSeen(channelID, batch)

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

func (m *DAGRunMonitor) buildNotificationMessage(ctx context.Context, sessionID string, user agent.UserIdentity, batch chatbridge.NotificationBatch, allowLLM bool) agent.Message {
	service := m.agentAPI
	if !allowLLM {
		service = nil
	}

	msg, err := chatbridge.GenerateNotificationMessage(ctx, service, sessionID, user, batch)
	if err != nil && len(batch.Events) > 0 && batch.Events[0].Status != nil {
		m.logger.Warn("Failed to generate AI notification, using deterministic fallback",
			slog.String("dag", batch.Events[0].Status.Name),
			slog.String("status", batch.Events[0].Status.Status.String()),
			slog.String("error", err.Error()),
		)
	}
	return msg
}

func (m *DAGRunMonitor) markBatchSeen(destination string, batch chatbridge.NotificationBatch) {
	for _, event := range batch.Events {
		m.markSeen(destination, event.Status)
	}
}

func (m *DAGRunMonitor) destinations() []string {
	destinations := make([]string, 0, len(m.bot.allowedChannels))
	for channelID := range m.bot.allowedChannels {
		destinations = append(destinations, channelID)
	}
	return destinations
}

func (m *DAGRunMonitor) setRunContext(ctx context.Context) {
	m.ctxMu.Lock()
	defer m.ctxMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	m.runCtx = ctx
}

func (m *DAGRunMonitor) runContext() context.Context {
	m.ctxMu.RLock()
	defer m.ctxMu.RUnlock()
	if m.runCtx == nil {
		return context.Background()
	}
	return m.runCtx
}

func (m *DAGRunMonitor) drainPendingBatches(ctx context.Context) {
	drained := m.batcher.DrainAndStop()
	if len(drained) == 0 {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notificationFlushTimeout)
	defer cancel()

	for _, pending := range drained {
		if shutdownCtx.Err() != nil {
			return
		}
		m.flushBatchWithPolicy(shutdownCtx, pending.Destination, pending.Batch, false)
	}
}
