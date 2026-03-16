// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package telegram

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
)

// monitorPollInterval is how often the monitor checks for DAG run status changes.
const monitorPollInterval = 10 * time.Second

// seenEvictInterval is how often stale entries are purged from the seen map.
const seenEvictInterval = 10 * time.Minute

// seenTTL is how long a seen entry is kept before eviction. This must be
// longer than the lookback window in checkForCompletions (1 hour).
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
// notifications via Telegram.
type DAGRunMonitor struct {
	dagRunStore exec.DAGRunStore
	agentAPI    AgentService
	bot         *Bot
	logger      *slog.Logger

	// seen tracks DAG runs we've already notified about (dagRunID+attemptID -> true)
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

// Run starts the monitor loop. It polls for completed DAG runs and sends
// AI-generated notifications to all allowed Telegram chats.
func (m *DAGRunMonitor) Run(ctx context.Context) {
	m.logger.Info("DAG run monitor started")

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

// seenKey returns a unique key for a DAG run status.
func seenKey(s *exec.DAGRunStatus) string {
	return s.DAGRunID + ":" + s.AttemptID
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

// notifyCompletion creates an agent session per chat to generate a notification
// about a completed DAG run. The session is adopted by the chat so the user
// can continue the conversation (e.g., ask follow-up questions about the run).
func (m *DAGRunMonitor) notifyCompletion(ctx context.Context, s *exec.DAGRunStatus) bool {
	m.logger.Info("DAG run completed, generating notification",
		slog.String("dag", s.Name),
		slog.String("status", s.Status.String()),
		slog.String("dag_run_id", s.DAGRunID),
	)

	if m.agentAPI == nil {
		m.sendFallbackNotification(s, "")
		return true // fallback always delivers
	}

	if len(m.bot.allowedChats) == 0 {
		m.logger.Warn("No allowed chats configured, cannot send notification")
		return false
	}

	prompt := buildNotificationPrompt(s)

	// Create a separate session for each chat so each user can follow up.
	allDelivered := true
	for chatID := range m.bot.allowedChats {
		if !m.notifyChat(ctx, chatID, s, prompt) {
			allDelivered = false
		}
	}
	return allDelivered
}

// notifyChat creates an agent session for a specific chat, waits for the
// response, sends it, and adopts the session so the user can follow up.
// Returns true if the notification was delivered successfully.
func (m *DAGRunMonitor) notifyChat(ctx context.Context, chatID int64, s *exec.DAGRunStatus, prompt string) bool {
	user := m.bot.userIdentityFromChatID(chatID)

	req := agent.ChatRequest{
		Message:  prompt,
		SafeMode: false,
	}

	sessionID, _, err := m.agentAPI.CreateSession(ctx, user, req)
	if err != nil {
		m.logger.Warn("Failed to create notification session",
			slog.String("dag", s.Name),
			slog.String("error", err.Error()),
		)
		m.bot.sendText(chatID, fmt.Sprintf("%s DAG '%s' %s\n(AI unavailable: %s)",
			statusEmoji(s.Status), s.Name, s.Status.String(), err.Error()))
		return false
	}

	// Wait for the agent to generate a response
	response := m.waitForAgentResponse(ctx, sessionID, user.UserID)
	if response == "" {
		m.bot.sendText(chatID, fmt.Sprintf("%s DAG '%s' %s\n(AI agent timed out)",
			statusEmoji(s.Status), s.Name, s.Status.String()))
		return false
	}

	// Adopt this session as the chat's active session so the user can
	// send follow-up messages (e.g., "show me the logs", "retry it").
	cs := m.bot.getOrCreateChat(chatID)
	m.bot.resetChat(cs)
	cs.mu.Lock()
	cs.sessionID = sessionID
	cs.ownerUserID = user.UserID
	cs.mu.Unlock()

	m.bot.sendLongText(chatID, response)

	// Start subscription so any further agent activity is forwarded
	m.bot.startSubscription(ctx, cs, chatID, user.UserID, sessionID)
	return true
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

	// Include step summary
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

// waitForAgentResponse polls the agent session for a response with a timeout.
func (m *DAGRunMonitor) waitForAgentResponse(ctx context.Context, sessionID, userID string) string {
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Track the latest assistant content across all polls, not just within
	// a single poll. The assistant message may arrive while Working is still
	// true, and by the next poll (when Working is false) there are no new
	// messages — so we must remember what we saw earlier.
	var latestAssistant string

	for {
		select {
		case <-ctx.Done():
			return ""
		case <-timeout:
			m.logger.Warn("Timeout waiting for agent notification response",
				slog.String("session", sessionID),
			)
			return ""
		case <-ticker.C:
			detail, err := m.agentAPI.GetSessionDetail(ctx, sessionID, userID)
			if err != nil || detail == nil {
				continue
			}

			// Scan all messages for the latest assistant content.
			for _, msg := range detail.Messages {
				if msg.Type == agent.MessageTypeAssistant && msg.Content != "" {
					latestAssistant = msg.Content
				}
			}

			// If agent is done working and we have a response, return it
			if detail.SessionState != nil && !detail.SessionState.Working {
				if latestAssistant != "" {
					return latestAssistant
				}
				// Agent finished but produced no text — give up
				m.logger.Warn("Agent finished without producing a text response",
					slog.String("session", sessionID),
				)
				return ""
			}
		}
	}
}

// sendFallbackNotification sends a simple non-AI notification to all chats.
func (m *DAGRunMonitor) sendFallbackNotification(s *exec.DAGRunStatus, reason string) {
	emoji := statusEmoji(s.Status)
	text := fmt.Sprintf("%s DAG '%s' %s", emoji, s.Name, s.Status.String())
	if s.Error != "" {
		text += "\nError: " + s.Error
	}
	if reason != "" {
		text += "\n" + reason
	}
	for chatID := range m.bot.allowedChats {
		m.bot.sendText(chatID, text)
	}
}

// statusEmoji returns an emoji for the DAG run status.
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
