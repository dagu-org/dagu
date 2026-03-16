// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package telegram provides a Telegram bot service that bridges Telegram
// chats with the Dagu AI agent, allowing users to interact with the agent
// via Telegram messages.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// maxTelegramMessageLen is the maximum length for a single Telegram message.
const maxTelegramMessageLen = 4096

// defaultContextLimit is the assumed context window size when no model config is available.
const defaultContextLimit = 200_000

// contextRotationRatio is the fraction of the context limit at which sessions are rotated.
const contextRotationRatio = 0.5

// AgentService is the subset of the agent API that the Telegram bot requires.
// Defining this as an interface decouples the bot from the concrete agent.API
// implementation, making it testable in isolation.
type AgentService interface {
	CreateSession(ctx context.Context, user agent.UserIdentity, req agent.ChatRequest) (string, string, error)
	SendMessage(ctx context.Context, sessionID string, user agent.UserIdentity, req agent.ChatRequest) error
	CancelSession(ctx context.Context, sessionID, userID string) error
	SubmitUserResponse(ctx context.Context, sessionID, userID string, resp agent.UserPromptResponse) error
	GetSessionDetail(ctx context.Context, sessionID, userID string) (*agent.StreamResponse, error)
	SubscribeSession(ctx context.Context, sessionID string, user agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error)
}

// Config holds configuration for the Telegram bot.
type Config struct {
	Token          string
	AllowedChatIDs []int64
	SafeMode       bool
	DAGRunStore    exec.DAGRunStore // optional: enables DAG run monitoring
}

// chatState tracks the agent session state for a single Telegram chat.
type chatState struct {
	sessionID       string
	subSessionID    string // session ID the subscription is listening to
	subCancel       context.CancelFunc
	mu              sync.Mutex
	pendingPromptID string
}

// Bot is a Telegram bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg          Config
	agentAPI     AgentService
	botAPI       *tgbotapi.BotAPI
	chats        sync.Map // chatID (int64) -> *chatState
	allowedChats map[int64]struct{}
	dagRunStore  exec.DAGRunStore
	logger       *slog.Logger
}

// New creates a new Telegram bot instance.
func New(cfg Config, agentAPI AgentService, logger *slog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required (set DAGU_TELEGRAM_TOKEN)")
	}
	if len(cfg.AllowedChatIDs) == 0 {
		return nil, fmt.Errorf("at least one allowed chat ID is required (set DAGU_TELEGRAM_ALLOWED_CHAT_IDS)")
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	allowed := make(map[int64]struct{}, len(cfg.AllowedChatIDs))
	for _, id := range cfg.AllowedChatIDs {
		allowed[id] = struct{}{}
	}

	return &Bot{
		cfg:          cfg,
		agentAPI:     agentAPI,
		botAPI:       botAPI,
		allowedChats: allowed,
		dagRunStore:  cfg.DAGRunStore,
		logger:       logger,
	}, nil
}

// Run starts the bot and blocks until the context is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Telegram bot started",
		slog.String("username", b.botAPI.Self.UserName),
		slog.Int("allowed_chats", len(b.allowedChats)),
	)

	// Start DAG run monitor if a DAGRunStore is available
	if b.dagRunStore != nil {
		monitor := NewDAGRunMonitor(b.dagRunStore, b.agentAPI, b, b.logger)
		go monitor.Run(ctx)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.botAPI.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.botAPI.StopReceivingUpdates()
			b.logger.Info("Telegram bot stopped")
			return nil

		case update := <-updates:
			if update.CallbackQuery != nil {
				b.handleCallbackQuery(ctx, update.CallbackQuery)
				continue
			}
			if update.Message == nil {
				continue
			}
			b.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage processes an incoming Telegram message.
func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if !b.isChatAllowed(chatID) {
		b.sendText(chatID, "This bot is not authorized for this chat.")
		return
	}

	// Handle commands
	if msg.IsCommand() {
		b.handleCommand(ctx, msg)
		return
	}

	text := msg.Text
	if text == "" {
		return
	}

	// Check if this is a response to a pending prompt
	cs := b.getOrCreateChat(chatID)
	cs.mu.Lock()
	pendingPrompt := cs.pendingPromptID
	cs.mu.Unlock()

	if pendingPrompt != "" {
		b.submitPromptResponse(ctx, cs, chatID, pendingPrompt, text)
		return
	}

	// Send message to agent
	user := b.userIdentity(msg)

	if cs.sessionID == "" {
		b.createSession(ctx, cs, chatID, user, text)
	} else {
		// Rotate session if approaching context limit
		if b.shouldRotateSession(ctx, cs, user.UserID) {
			b.rotateSession(ctx, cs, chatID, user, text)
		} else {
			b.sendMessage(ctx, cs, chatID, user, text)
		}
	}
}

// handleCommand processes bot commands.
func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	switch msg.Command() {
	case "new":
		cs := b.getOrCreateChat(chatID)
		b.resetChat(cs)
		b.sendText(chatID, "Session cleared. Send a message to start a new conversation.")

	case "cancel":
		cs := b.getOrCreateChat(chatID)
		cs.mu.Lock()
		sid := cs.sessionID
		cs.mu.Unlock()

		if sid == "" {
			b.sendText(chatID, "No active session.")
			return
		}

		user := b.userIdentity(msg)
		if err := b.agentAPI.CancelSession(ctx, sid, user.UserID); err != nil {
			b.sendText(chatID, "Failed to cancel session: "+err.Error())
			return
		}
		b.sendText(chatID, "Session cancelled.")

	case "start":
		b.sendText(chatID, "Welcome to Dagu AI Agent! Send any message to start chatting.\n\nCommands:\n/new - Start a new session\n/cancel - Cancel current session")

	default:
		b.sendText(chatID, "Unknown command. Use /new, /cancel, or just send a message.")
	}
}

// handleCallbackQuery processes inline keyboard button presses.
func (b *Bot) handleCallbackQuery(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID

	// Parse callback data: "prompt:<promptID>:<optionID>"
	parts := strings.SplitN(cq.Data, ":", 3)
	if len(parts) != 3 || parts[0] != "prompt" {
		callback := tgbotapi.NewCallback(cq.ID, "Invalid action")
		_, _ = b.botAPI.Send(callback)
		return
	}

	promptID := parts[1]
	optionID := parts[2]

	cs := b.getOrCreateChat(chatID)
	cs.mu.Lock()
	sid := cs.sessionID
	cs.pendingPromptID = ""
	cs.mu.Unlock()

	if sid == "" {
		callback := tgbotapi.NewCallback(cq.ID, "No active session")
		_, _ = b.botAPI.Send(callback)
		return
	}

	user := b.userIdentityFromCallback(cq)
	resp := agent.UserPromptResponse{
		PromptID:          promptID,
		SelectedOptionIDs: []string{optionID},
	}

	if err := b.agentAPI.SubmitUserResponse(ctx, sid, user.UserID, resp); err != nil {
		b.logger.Warn("Failed to submit prompt response",
			slog.String("session", sid),
			slog.String("user", user.UserID),
			slog.String("prompt", promptID),
			slog.String("error", err.Error()),
		)
		// Send the actual error to Telegram so user can see what went wrong
		b.sendText(chatID, fmt.Sprintf("Failed to submit response: %s", err.Error()))
		callback := tgbotapi.NewCallback(cq.ID, "Error - see message")
		_, _ = b.botAPI.Send(callback)
		return
	}

	// Acknowledge and update the message
	callback := tgbotapi.NewCallback(cq.ID, "Response submitted")
	_, _ = b.botAPI.Send(callback)

	// Edit the original message to show the selection
	edit := tgbotapi.NewEditMessageText(chatID, cq.Message.MessageID,
		cq.Message.Text+"\n\nSelected: "+optionID)
	_, _ = b.botAPI.Send(edit)
}

// sendTyping sends a "typing..." indicator to the Telegram chat.
func (b *Bot) sendTyping(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, _ = b.botAPI.Send(action)
}

// createSession creates a new agent session and starts listening for responses.
func (b *Bot) createSession(ctx context.Context, cs *chatState, chatID int64, user agent.UserIdentity, text string) {
	b.sendTyping(chatID)

	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	sessionID, _, err := b.agentAPI.CreateSession(ctx, user, req)
	if err != nil {
		b.logger.Error("Failed to create session", slog.String("error", err.Error()))
		b.sendText(chatID, "Failed to start session: "+err.Error())
		return
	}

	cs.mu.Lock()
	cs.sessionID = sessionID
	cs.mu.Unlock()

	b.startSubscription(ctx, cs, chatID, user.UserID, sessionID)
}

// sendMessage sends a message to an existing session.
func (b *Bot) sendMessage(ctx context.Context, cs *chatState, chatID int64, user agent.UserIdentity, text string) {
	b.sendTyping(chatID)

	cs.mu.Lock()
	sid := cs.sessionID
	cs.mu.Unlock()

	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	if err := b.agentAPI.SendMessage(ctx, sid, user, req); err != nil {
		b.logger.Error("Failed to send message", slog.String("error", err.Error()))
		b.sendText(chatID, "Failed to send message: "+err.Error())
		return
	}

	// Ensure a subscription is running (but don't restart if already active for this session)
	b.ensureSubscription(ctx, cs, chatID, user.UserID, sid)
}

// submitPromptResponse submits a text response to a pending agent prompt.
func (b *Bot) submitPromptResponse(ctx context.Context, cs *chatState, chatID int64, promptID, text string) {
	cs.mu.Lock()
	sid := cs.sessionID
	cs.pendingPromptID = ""
	cs.mu.Unlock()

	if sid == "" {
		return
	}

	user := b.userIdentityFromChatID(chatID)
	resp := agent.UserPromptResponse{
		PromptID:         promptID,
		FreeTextResponse: text,
	}

	if err := b.agentAPI.SubmitUserResponse(ctx, sid, user.UserID, resp); err != nil {
		b.sendText(chatID, "Failed to submit response: "+err.Error())
	}
}

// ensureSubscription starts a subscription only if one isn't already running
// for the given session. Safe to call on every message.
func (b *Bot) ensureSubscription(ctx context.Context, cs *chatState, chatID int64, userID, sessionID string) {
	cs.mu.Lock()
	if cs.subSessionID == sessionID && cs.subCancel != nil {
		// Already subscribed to this session — nothing to do.
		cs.mu.Unlock()
		return
	}
	// Different session or no subscription — start a new one.
	if cs.subCancel != nil {
		cs.subCancel()
	}
	subCtx, cancel := context.WithCancel(ctx)
	cs.subCancel = cancel
	cs.subSessionID = sessionID
	cs.mu.Unlock()

	go b.subscribeLoop(subCtx, cs, chatID, userID, sessionID)
}

// startSubscription cancels any existing subscription and starts a fresh one.
// Use this when creating a new session or adopting a notification session.
func (b *Bot) startSubscription(ctx context.Context, cs *chatState, chatID int64, userID, sessionID string) {
	cs.mu.Lock()
	if cs.subCancel != nil {
		cs.subCancel()
	}
	subCtx, cancel := context.WithCancel(ctx)
	cs.subCancel = cancel
	cs.subSessionID = sessionID
	cs.mu.Unlock()

	go b.subscribeLoop(subCtx, cs, chatID, userID, sessionID)
}

// subscribeLoop uses the agent's built-in pub-sub to receive session updates
// in real time and forward them to Telegram.
func (b *Bot) subscribeLoop(ctx context.Context, cs *chatState, chatID int64, userID, sessionID string) {
	user := agent.UserIdentity{
		UserID:   userID,
		Username: "telegram",
		Role:     auth.RoleAdmin,
	}

	snapshot, next, err := b.agentAPI.SubscribeSession(ctx, sessionID, user)
	if err != nil {
		b.logger.Warn("Failed to subscribe to session",
			slog.String("session", sessionID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Process the initial snapshot — skip user messages (we sent those),
	// but forward any assistant/error/prompt messages that arrived before
	// the subscription started.
	b.processStreamResponse(cs, chatID, snapshot)

	// Listen for real-time updates via the pub-sub channel.
	for {
		resp, ok := next()
		if !ok {
			return
		}
		b.processStreamResponse(cs, chatID, resp)
	}
}

// processStreamResponse handles a stream response and sends relevant content to Telegram.
func (b *Bot) processStreamResponse(cs *chatState, chatID int64, resp agent.StreamResponse) {
	for _, msg := range resp.Messages {
		switch msg.Type {
		case agent.MessageTypeAssistant:
			if msg.Content != "" {
				b.sendLongText(chatID, msg.Content)
			}

		case agent.MessageTypeError:
			if msg.Content != "" {
				b.sendText(chatID, "Error: "+msg.Content)
			}

		case agent.MessageTypeUserPrompt:
			if msg.UserPrompt != nil {
				b.sendPrompt(cs, chatID, msg.UserPrompt)
			}

		case agent.MessageTypeUser, agent.MessageTypeUIAction:
			// Skip user messages and UI actions in Telegram output
		}
	}
}

// sendPrompt sends a user prompt with inline keyboard buttons.
func (b *Bot) sendPrompt(cs *chatState, chatID int64, prompt *agent.UserPrompt) {
	cs.mu.Lock()
	cs.pendingPromptID = prompt.PromptID
	cs.mu.Unlock()

	text := prompt.Question
	if prompt.Command != "" {
		text += "\n\nCommand: `" + prompt.Command + "`"
	}
	if prompt.AllowFreeText {
		text += "\n\n(You can also reply with text)"
	}

	if len(prompt.Options) > 0 {
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, opt := range prompt.Options {
			callbackData := fmt.Sprintf("prompt:%s:%s", prompt.PromptID, opt.ID)
			label := opt.Label
			if opt.Description != "" {
				label += " - " + opt.Description
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, callbackData),
			))
		}

		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.botAPI.Send(msg); err != nil {
			b.logger.Warn("Failed to send prompt", slog.String("error", err.Error()))
		}
	} else {
		b.sendText(chatID, text)
	}
}

// sendLongText sends a message, splitting it if it exceeds Telegram's limit.
func (b *Bot) sendLongText(chatID int64, text string) {
	chunks := splitMessage(text, maxTelegramMessageLen)
	for _, chunk := range chunks {
		b.sendText(chatID, chunk)
	}
}

// sendText sends a simple text message to a Telegram chat.
func (b *Bot) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.botAPI.Send(msg); err != nil {
		b.logger.Warn("Failed to send Telegram message",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()),
		)
	}
}

// isChatAllowed checks if a chat ID is authorized to use the bot.
func (b *Bot) isChatAllowed(chatID int64) bool {
	_, ok := b.allowedChats[chatID]
	return ok
}

// getOrCreateChat returns or creates a chatState for the given chat ID.
func (b *Bot) getOrCreateChat(chatID int64) *chatState {
	val, _ := b.chats.LoadOrStore(chatID, &chatState{})
	return val.(*chatState)
}

// resetChat clears the session state for a chat.
func (b *Bot) resetChat(cs *chatState) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.subCancel != nil {
		cs.subCancel()
		cs.subCancel = nil
	}
	cs.sessionID = ""
	cs.subSessionID = ""
	cs.pendingPromptID = ""
}

// userIdentity creates a UserIdentity from a Telegram message.
func (b *Bot) userIdentity(msg *tgbotapi.Message) agent.UserIdentity {
	username := msg.From.UserName
	if username == "" {
		username = strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	}
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("telegram:%d", msg.From.ID),
		Username: username,
		Role:     auth.RoleAdmin,
	}
}

// userIdentityFromCallback creates a UserIdentity from a callback query.
func (b *Bot) userIdentityFromCallback(cq *tgbotapi.CallbackQuery) agent.UserIdentity {
	username := cq.From.UserName
	if username == "" {
		username = strings.TrimSpace(cq.From.FirstName + " " + cq.From.LastName)
	}
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("telegram:%d", cq.From.ID),
		Username: username,
		Role:     auth.RoleAdmin,
	}
}

// userIdentityFromChatID creates a minimal UserIdentity from a chat ID.
func (b *Bot) userIdentityFromChatID(chatID int64) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("telegram:%d", chatID),
		Username: "telegram",
		Role:     auth.RoleAdmin,
	}
}

// splitMessage splits text into chunks that fit within the Telegram message limit.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Try to split at a paragraph boundary
		cut := maxLen
		if idx := strings.LastIndex(text[:maxLen], "\n\n"); idx > maxLen/2 {
			cut = idx + 2
		} else if idx := strings.LastIndex(text[:maxLen], "\n"); idx > maxLen/2 {
			cut = idx + 1
		}

		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}

	return chunks
}

// rotateSession creates a new session carrying forward recent context from the
// old session. The user's new message is prepended with a summary of the recent
// conversation so the agent doesn't lose context.
func (b *Bot) rotateSession(ctx context.Context, cs *chatState, chatID int64, user agent.UserIdentity, text string) {
	cs.mu.Lock()
	oldSID := cs.sessionID
	cs.mu.Unlock()

	// Collect the last few assistant messages as context summary
	var summary string
	if oldSID != "" {
		summary = b.buildSessionSummary(ctx, oldSID, user.UserID)
	}

	b.resetChat(cs)
	b.sendText(chatID, "(Session context limit reached — continuing with recent context carried forward)")

	// Prepend the summary to the user's message so the new session has context
	var message string
	if summary != "" {
		message = fmt.Sprintf("[Previous conversation summary:\n%s]\n\n%s", summary, text)
	} else {
		message = text
	}

	b.createSession(ctx, cs, chatID, user, message)
}

// buildSessionSummary extracts the last few assistant messages from a session
// to use as context when rotating to a new session.
func (b *Bot) buildSessionSummary(ctx context.Context, sessionID, userID string) string {
	detail, err := b.agentAPI.GetSessionDetail(ctx, sessionID, userID)
	if err != nil || detail == nil {
		return ""
	}

	// Collect the last few user+assistant exchanges (up to 3 pairs)
	const maxExchanges = 3
	var exchanges []string
	var count int

	// Walk backwards through messages
	for i := len(detail.Messages) - 1; i >= 0 && count < maxExchanges; i-- {
		msg := detail.Messages[i]
		switch msg.Type {
		case agent.MessageTypeAssistant:
			if msg.Content != "" {
				content := msg.Content
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				exchanges = append([]string{"Assistant: " + content}, exchanges...)
			}
		case agent.MessageTypeUser:
			if msg.Content != "" {
				content := msg.Content
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				exchanges = append([]string{"User: " + content}, exchanges...)
				count++
			}
		case agent.MessageTypeError, agent.MessageTypeUIAction, agent.MessageTypeUserPrompt:
			// Skip non-conversational messages in summary
		}
	}

	return strings.Join(exchanges, "\n")
}

// shouldRotateSession checks if the session's token usage has reached
// 50% of the context limit and should be rotated to a fresh session.
func (b *Bot) shouldRotateSession(ctx context.Context, cs *chatState, userID string) bool {
	if b.agentAPI == nil {
		return false
	}

	cs.mu.Lock()
	sid := cs.sessionID
	cs.mu.Unlock()

	if sid == "" {
		return false
	}

	detail, err := b.agentAPI.GetSessionDetail(ctx, sid, userID)
	if err != nil || detail == nil {
		return false
	}

	// Sum total tokens across all messages
	var totalTokens int
	for _, msg := range detail.Messages {
		if msg.Usage != nil {
			totalTokens += msg.Usage.TotalTokens
		}
	}

	limit := int(float64(defaultContextLimit) * contextRotationRatio)
	return totalTokens > limit
}
