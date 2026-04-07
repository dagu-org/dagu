// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/chatbridge"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

// DAGRunMonitor watches for DAG run completions and sends notifications via Telegram.
type DAGRunMonitor struct {
	core     *chatbridge.NotificationMonitor
	agentAPI AgentService
	bot      *Bot
	logger   *slog.Logger
}

// NewDAGRunMonitor creates a new monitor instance.
func NewDAGRunMonitor(eventService *eventstore.Service, stateFile string, agentAPI AgentService, bot *Bot, logger *slog.Logger) *DAGRunMonitor {
	return newDAGRunMonitorWithWindows(
		eventService,
		stateFile,
		agentAPI,
		bot,
		logger,
		chatbridge.DefaultUrgentNotificationWindow,
		chatbridge.DefaultSuccessNotificationWindow,
	)
}

func newDAGRunMonitorWithWindows(
	eventService *eventstore.Service,
	stateFile string,
	agentAPI AgentService,
	bot *Bot,
	logger *slog.Logger,
	urgentWindow, successWindow time.Duration,
) *DAGRunMonitor {
	monitor := &DAGRunMonitor{
		agentAPI: agentAPI,
		bot:      bot,
		logger:   logger,
	}
	cfg := chatbridge.DefaultNotificationMonitorConfig()
	cfg.UrgentWindow = urgentWindow
	cfg.SuccessWindow = successWindow
	cfg.InterestedEventTypes = telegramInterestedEventTypes(bot.cfg.InterestedEventTypes)
	monitor.core = chatbridge.NewNotificationMonitor(eventService, stateFile, monitor, logger, cfg)
	return monitor
}

func telegramInterestedEventTypes(values []string) []eventstore.EventType {
	if values == nil {
		return nil
	}
	result := make([]eventstore.EventType, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		result = append(result, eventstore.EventType(value))
	}
	return result
}

// Run starts the monitor loop.
func (m *DAGRunMonitor) Run(ctx context.Context) {
	m.core.Run(ctx)
}

func (m *DAGRunMonitor) notifyCompletion(_ context.Context, s *exec.DAGRunStatus) bool {
	return m.core.NotifyCompletion(s)
}

func (m *DAGRunMonitor) isSeen(destination string, s *exec.DAGRunStatus) bool {
	return m.core.IsDelivered(destination, s)
}

// NotificationDestinations returns the configured Telegram chat IDs as strings.
func (m *DAGRunMonitor) NotificationDestinations() []string {
	destinations := make([]string, 0, len(m.bot.allowedChats))
	for chatID := range m.bot.allowedChats {
		destinations = append(destinations, strconv.FormatInt(chatID, 10))
	}
	return destinations
}

// FlushNotificationBatch delivers a single notification batch to Telegram.
func (m *DAGRunMonitor) FlushNotificationBatch(ctx context.Context, chatIDKey string, batch chatbridge.NotificationBatch, allowLLM bool) bool {
	chatID, err := strconv.ParseInt(chatIDKey, 10, 64)
	if err != nil {
		m.logger.Warn("Failed to parse Telegram chat ID for notification batch",
			slog.String("chat_id", chatIDKey),
			slog.String("error", err.Error()),
		)
		return false
	}

	return m.flushChat(ctx, chatID, batch, allowLLM)
}

// flushChat appends the notification batch into the chat-scoped session.
func (m *DAGRunMonitor) flushChat(ctx context.Context, chatID int64, batch chatbridge.NotificationBatch, allowLLM bool) bool {
	user := m.bot.userIdentity(chatID)
	cs := m.bot.getOrCreateChat(chatID)
	sessionID := m.currentSessionID(cs)
	msg := m.buildNotificationMessage(ctx, sessionID, user, batch, allowLLM)
	sessionID, stored, ok := m.appendNotification(ctx, cs, sessionID, user, chatbridge.NotificationBatchTopicName(batch), msg)
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

func (m *DAGRunMonitor) buildNotificationMessage(ctx context.Context, sessionID string, user agent.UserIdentity, batch chatbridge.NotificationBatch, allowLLM bool) agent.Message {
	service := m.agentAPI
	if !allowLLM {
		service = nil
	}

	msg, err := chatbridge.GenerateNotificationMessage(ctx, service, sessionID, user, batch)
	if err != nil && len(batch.Events) > 0 {
		subject := chatbridge.NotificationBatchTopicName(batch)
		status := ""
		if len(batch.Events) > 0 {
			status = chatbridge.NotificationStatusLabel(batch.Events[0])
		}
		m.logger.Warn("Failed to generate AI notification, using deterministic fallback",
			slog.String("subject", subject),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
	return msg
}
