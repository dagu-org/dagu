// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
)

// monitorPollInterval is how often the monitor checks for DAG run status changes.
const monitorPollInterval = 10 * time.Second

// seenEvictInterval is how often stale entries are purged from the seen map.
const seenEvictInterval = 10 * time.Minute

// seenTTL is how long a seen entry is kept before eviction.
const seenTTL = 2 * time.Hour

// notifyStatuses are the statuses that trigger a notification.
var notifyStatuses = []core.Status{
	core.Succeeded,
	core.Failed,
	core.Aborted,
	core.PartiallySucceeded,
	core.Rejected,
	core.Waiting,
}

// DAGRunMonitor watches for DAG run completions and sends AI-generated
// notifications via Slack.
type DAGRunMonitor struct {
	dagRunStore exec.DAGRunStore
	agentAPI    AgentService
	bot         *Bot
	logger      *slog.Logger

	seen sync.Map
}

// NewDAGRunMonitor creates a new monitor instance.
func NewDAGRunMonitor(dagRunStore exec.DAGRunStore, agentAPI AgentService, bot *Bot, logger *slog.Logger) *DAGRunMonitor {
	return &DAGRunMonitor{
		dagRunStore: dagRunStore,
		agentAPI:    agentAPI,
		bot:         bot,
		logger:      logger,
	}
}

// Run starts the monitor loop.
func (m *DAGRunMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run monitor started")

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
		exec.WithStatuses(notifyStatuses),
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
	return s.DAGRunID + ":" + s.AttemptID
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

// notifyCompletion appends a notification into the active conversation for each channel.
func (m *DAGRunMonitor) notifyCompletion(ctx context.Context, s *exec.DAGRunStatus) bool {
	m.logger.Info("DAG run completed, generating notification",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("dag_run_id", s.DAGRunID),
	)

	if m.agentAPI == nil {
		m.sendFallbackNotification(s, "")
		return true
	}

	if len(m.bot.allowedChannels) == 0 {
		m.logger.Warn("No allowed channels configured, cannot send notification")
		return false
	}

	prompt := buildNotificationPrompt(s)

	for channelID := range m.bot.allowedChannels {
		m.notifyChannel(ctx, channelID, s, prompt)
	}
	return true // Mark as seen even on partial failure to avoid duplicates
}

// notifyChannel appends a notification to either the DM conversation or a new
// per-notification thread in a Slack channel.
func (m *DAGRunMonitor) notifyChannel(ctx context.Context, channelID string, s *exec.DAGRunStatus, prompt string) bool {
	if strings.HasPrefix(channelID, "D") {
		return m.notifyDirectMessage(ctx, channelID, s, prompt)
	}
	return m.notifyChannelThread(ctx, channelID, s, prompt)
}

func (m *DAGRunMonitor) notifyDirectMessage(ctx context.Context, channelID string, s *exec.DAGRunStatus, prompt string) bool {
	convKey := channelID
	user := m.bot.userIdentity(convKey)
	cs := m.bot.getOrCreateChat(convKey, channelID, "")
	sessionID := m.currentSessionID(cs)

	msg := m.buildNotificationMessage(ctx, sessionID, user, s, prompt)
	sessionID, stored, ok := m.appendNotification(ctx, cs, sessionID, user, s.Name, msg)
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

func (m *DAGRunMonitor) notifyChannelThread(ctx context.Context, channelID string, s *exec.DAGRunStatus, prompt string) bool {
	msg := m.buildNotificationMessage(ctx, "", m.bot.userIdentity(channelID), s, prompt)
	threadTS := m.bot.sendLongRootThread(channelID, msg.Content)
	if threadTS == "" {
		return false
	}

	threadKey := channelID + ":" + threadTS
	m.bot.activeThreads.Store(threadKey, true)

	cs := m.bot.getOrCreateChat(threadKey, channelID, threadTS)
	user := m.bot.userIdentity(threadKey)

	sessionID, err := m.agentAPI.CreateEmptySession(ctx, user, s.Name, m.bot.cfg.SafeMode)
	if err != nil {
		m.logger.Warn("Failed to create threaded notification session",
			slog.String("dag", s.Name),
			slog.String("channel", channelID),
			slog.String("thread_ts", threadTS),
			slog.String("error", err.Error()),
		)
		return true
	}
	m.bot.setActiveSession(cs, sessionID, user.UserID)

	stored, err := m.agentAPI.AppendExternalMessage(ctx, sessionID, user, msg)
	if err != nil {
		m.logger.Warn("Failed to append threaded notification message",
			slog.String("session", sessionID),
			slog.String("dag", s.Name),
			slog.String("error", err.Error()),
		)
		return true
	}
	m.bot.markDelivered(cs, stored.SequenceID)
	return true
}

func (m *DAGRunMonitor) currentSessionID(cs *chatState) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sessionID
}

func (m *DAGRunMonitor) appendNotification(ctx context.Context, cs *chatState, sessionID string, user agent.UserIdentity, dagName string, msg agent.Message) (string, agent.Message, bool) {
	appendToSession := func(targetSessionID string) (agent.Message, error) {
		return m.agentAPI.AppendExternalMessage(ctx, targetSessionID, user, msg)
	}

	if sessionID == "" {
		newSessionID, err := m.agentAPI.CreateEmptySession(ctx, user, dagName, m.bot.cfg.SafeMode)
		if err != nil {
			m.logger.Warn("Failed to create notification session",
				slog.String("dag", dagName),
				slog.String("user", user.UserID),
				slog.String("error", err.Error()),
			)
			return "", agent.Message{}, false
		}
		m.bot.setActiveSession(cs, newSessionID, user.UserID)
		stored, err := appendToSession(newSessionID)
		if err != nil {
			m.logger.Warn("Failed to append notification message to new session",
				slog.String("session", newSessionID),
				slog.String("error", err.Error()),
			)
			return "", agent.Message{}, false
		}
		return newSessionID, stored, true
	}

	stored, err := appendToSession(sessionID)
	if err == nil {
		return sessionID, stored, true
	}
	if err != agent.ErrSessionNotFound {
		m.logger.Warn("Failed to append notification message",
			slog.String("session", sessionID),
			slog.String("error", err.Error()),
		)
		return "", agent.Message{}, false
	}

	newSessionID, createErr := m.agentAPI.CreateEmptySession(ctx, user, dagName, m.bot.cfg.SafeMode)
	if createErr != nil {
		m.logger.Warn("Failed to replace missing notification session",
			slog.String("dag", dagName),
			slog.String("user", user.UserID),
			slog.String("error", createErr.Error()),
		)
		return "", agent.Message{}, false
	}
	m.bot.setActiveSession(cs, newSessionID, user.UserID)
	stored, err = appendToSession(newSessionID)
	if err != nil {
		m.logger.Warn("Failed to append notification message after session recreation",
			slog.String("session", newSessionID),
			slog.String("error", err.Error()),
		)
		return "", agent.Message{}, false
	}
	return newSessionID, stored, true
}

func (m *DAGRunMonitor) buildNotificationMessage(ctx context.Context, sessionID string, user agent.UserIdentity, s *exec.DAGRunStatus, prompt string) agent.Message {
	msg, err := m.agentAPI.GenerateAssistantMessage(ctx, sessionID, user, s.Name, prompt)
	if err == nil {
		return msg
	}

	m.logger.Warn("Failed to generate AI notification, falling back to plain text",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("error", err.Error()),
	)
	text := fallbackNotificationText(s, "AI unavailable: "+err.Error())
	return agent.Message{
		Type:      agent.MessageTypeAssistant,
		Content:   text,
		CreatedAt: time.Now(),
		LLMData: &llm.Message{
			Role:    llm.RoleAssistant,
			Content: text,
		},
	}
}

// buildNotificationPrompt creates a prompt for the AI agent to analyze
// a completed DAG run and generate a user-friendly notification.
func buildNotificationPrompt(s *exec.DAGRunStatus) string {
	var intro string
	if s.Status == core.Waiting {
		intro = "A DAG run is waiting for human approval. Please write a brief, urgent notification message for the user. Let them know which steps are waiting and that action is needed. Keep it concise (2-4 sentences)."
	} else {
		intro = "A DAG run just completed. Please write a brief, helpful notification message for the user about this event. Keep it concise (2-4 sentences). Include the key facts and any actionable information."
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, `%s

DAG Name: %s
Status: %s
DAG Run ID: %s`, intro, s.Name, s.Status.String(), s.DAGRunID)

	if s.Error != "" {
		fmt.Fprintf(&prompt, "\nError: %s", s.Error)
	}
	if s.StartedAt != "" {
		fmt.Fprintf(&prompt, "\nStarted: %s", s.StartedAt)
	}
	if s.FinishedAt != "" {
		fmt.Fprintf(&prompt, "\nFinished: %s", s.FinishedAt)
	}
	if s.Log != "" {
		fmt.Fprintf(&prompt, "\nLog file: %s", s.Log)
	}

	if len(s.Nodes) > 0 {
		prompt.WriteString("\n\nStep results:")
		for _, n := range s.Nodes {
			line := fmt.Sprintf("\n- %s: %s", n.Step.Name, n.Status.String())
			if n.Error != "" {
				line += fmt.Sprintf(" (error: %s)", n.Error)
			}
			prompt.WriteString(line)
		}
	}

	prompt.WriteString("\n\nWrite a notification message. Do NOT use tools or execute any commands. Just write the message text directly.")

	return prompt.String()
}

// sendFallbackNotification sends a simple non-AI notification to all channels.
func (m *DAGRunMonitor) sendFallbackNotification(s *exec.DAGRunStatus, reason string) {
	text := fallbackNotificationText(s, reason)
	for channelID := range m.bot.allowedChannels {
		m.bot.sendText(channelID, text)
	}
}

func fallbackNotificationText(s *exec.DAGRunStatus, reason string) string {
	emoji := statusEmoji(s.Status)
	text := fmt.Sprintf("%s DAG '%s' %s", emoji, s.Name, s.Status.String())
	if s.Error != "" {
		text += "\nError: " + s.Error
	}
	if reason != "" {
		text += "\n" + reason
	}
	return text
}

func statusEmoji(s core.Status) string {
	switch s { //nolint:exhaustive // only notified statuses are handled
	case core.Succeeded, core.PartiallySucceeded:
		return "\u2705" // green check
	case core.Failed, core.Rejected:
		return "\u274C" // red X
	case core.Aborted:
		return "\u26A0\uFE0F" // warning
	case core.Waiting:
		return "\u23F3" // hourglass
	default:
		return "\u2139\uFE0F" // info
	}
}
