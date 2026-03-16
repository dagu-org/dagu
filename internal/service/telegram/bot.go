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
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// maxTelegramMessageLen is the maximum length for a single Telegram message.
const maxTelegramMessageLen = 4096

// Config holds configuration for the Telegram bot.
type Config struct {
	Token          string
	AllowedChatIDs []int64
	SafeMode       bool
	DAGRunStore    exec.DAGRunStore // optional: enables DAG run monitoring
}

// chatState tracks the agent session state for a single Telegram chat.
type chatState struct {
	sessionID  string
	subCancel  context.CancelFunc
	mu         sync.Mutex
	pendingPromptID string
}

// Bot is a Telegram bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg          Config
	agentAPI     *agent.API
	botAPI       *tgbotapi.BotAPI
	chats        sync.Map // chatID (int64) -> *chatState
	allowedChats map[int64]struct{}
	dagRunStore  exec.DAGRunStore
	logger       *slog.Logger
}

// New creates a new Telegram bot instance.
func New(cfg Config, agentAPI *agent.API, logger *slog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required (set DAGU_TELEGRAM_TOKEN)")
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
		b.sendMessage(ctx, cs, chatID, user, text)
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
		b.logger.Warn("Failed to submit prompt response", slog.String("error", err.Error()))
		callback := tgbotapi.NewCallback(cq.ID, "Failed to submit response")
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

// createSession creates a new agent session and starts listening for responses.
func (b *Bot) createSession(ctx context.Context, cs *chatState, chatID int64, user agent.UserIdentity, text string) {
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

	// Restart subscription if needed
	b.startSubscription(ctx, cs, chatID, user.UserID, sid)
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

// startSubscription starts a goroutine that subscribes to session updates
// and forwards them to the Telegram chat.
func (b *Bot) startSubscription(ctx context.Context, cs *chatState, chatID int64, userID, sessionID string) {
	cs.mu.Lock()
	// Cancel any existing subscription
	if cs.subCancel != nil {
		cs.subCancel()
	}
	subCtx, cancel := context.WithCancel(ctx)
	cs.subCancel = cancel
	cs.mu.Unlock()

	go b.subscribeLoop(subCtx, cs, chatID, userID, sessionID)
}

// subscribeLoop listens for session updates and sends them to Telegram.
func (b *Bot) subscribeLoop(ctx context.Context, cs *chatState, chatID int64, userID, sessionID string) {
	// Get the session detail to check current state
	detail, err := b.agentAPI.GetSessionDetail(ctx, sessionID, userID)
	if err != nil {
		b.logger.Warn("Failed to get session detail for subscription",
			slog.String("session", sessionID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Process any existing messages from the snapshot
	if detail != nil {
		b.processStreamResponse(cs, chatID, agent.StreamResponse{
			Messages:     detail.Messages,
			SessionState: detail.SessionState,
		})
	}

	// Poll for updates using GetSessionDetail periodically
	// The agent API doesn't expose direct subscription access outside HTTP,
	// so we poll for new messages.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastSeqID int64
	if detail != nil && len(detail.Messages) > 0 {
		lastSeqID = detail.Messages[len(detail.Messages)-1].SequenceID
	}

	idleCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			detail, err := b.agentAPI.GetSessionDetail(ctx, sessionID, userID)
			if err != nil {
				continue
			}
			if detail == nil {
				continue
			}

			// Find new messages
			var newMsgs []agent.Message
			for _, msg := range detail.Messages {
				if msg.SequenceID > lastSeqID {
					newMsgs = append(newMsgs, msg)
					lastSeqID = msg.SequenceID
				}
			}

			if len(newMsgs) > 0 {
				idleCount = 0
				b.processStreamResponse(cs, chatID, agent.StreamResponse{
					Messages:     newMsgs,
					SessionState: detail.SessionState,
				})
			} else {
				idleCount++
			}

			// Stop polling if not working and idle for a while
			if detail.SessionState != nil && !detail.SessionState.Working && idleCount > 3 {
				return
			}
		}
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
	if len(b.allowedChats) == 0 {
		return true
	}
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
