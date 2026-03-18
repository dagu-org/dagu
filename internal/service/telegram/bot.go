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
const defaultIncomingBatchDelay = 750 * time.Millisecond

// AgentService is the subset of the agent API that the Telegram bot requires.
// Defining this as an interface decouples the bot from the concrete agent.API
// implementation, making it testable in isolation.
type AgentService interface {
	CreateSession(ctx context.Context, user agent.UserIdentity, req agent.ChatRequest) (string, string, error)
	CreateEmptySession(ctx context.Context, user agent.UserIdentity, dagName string, safeMode bool) (string, error)
	SendMessage(ctx context.Context, sessionID string, user agent.UserIdentity, req agent.ChatRequest) error
	CancelSession(ctx context.Context, sessionID, userID string) error
	SubmitUserResponse(ctx context.Context, sessionID, userID string, resp agent.UserPromptResponse) error
	GenerateAssistantMessage(ctx context.Context, sessionID string, user agent.UserIdentity, dagName, prompt string) (agent.Message, error)
	AppendExternalMessage(ctx context.Context, sessionID string, user agent.UserIdentity, msg agent.Message) (agent.Message, error)
	CompactSessionIfNeeded(ctx context.Context, sessionID string, user agent.UserIdentity) (string, bool, error)
	GetSessionDetail(ctx context.Context, sessionID, userID string) (*agent.StreamResponse, error)
	SubscribeSession(ctx context.Context, sessionID string, user agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error)
}

type telegramAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
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
	sessionID        string
	ownerUserID      string // conversation-scoped session identity
	subSessionID     string // session ID the subscription is listening to
	subCancel        context.CancelFunc
	mu               sync.Mutex
	pendingPromptID  string
	lastDeliveredSeq int64
	pendingMessages  []string
	pendingFlushGen  uint64
}

// Bot is a Telegram bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg           Config
	agentAPI      AgentService
	botAPI        telegramAPI
	selfUsername  string
	chats         sync.Map // chatID (int64) -> *chatState
	allowedChats  map[int64]struct{}
	dagRunStore   exec.DAGRunStore
	logger        *slog.Logger
	incomingDelay time.Duration
}

// New creates a new Telegram bot instance.
func New(cfg Config, agentAPI AgentService, logger *slog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required (set DAGU_BOTS_TELEGRAM_TOKEN)")
	}
	if len(cfg.AllowedChatIDs) == 0 {
		return nil, fmt.Errorf("at least one allowed chat ID is required (set DAGU_BOTS_TELEGRAM_ALLOWED_CHAT_IDS)")
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
		cfg:           cfg,
		agentAPI:      agentAPI,
		botAPI:        botAPI,
		selfUsername:  botAPI.Self.UserName,
		allowedChats:  allowed,
		dagRunStore:   cfg.DAGRunStore,
		logger:        logger,
		incomingDelay: defaultIncomingBatchDelay,
	}, nil
}

// Run starts the bot and blocks until the context is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Telegram bot started",
		slog.String("username", b.selfUsername),
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
	b.enqueueIncomingMessage(ctx, cs, chatID, text)
}

// handleCommand processes bot commands.
func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	switch msg.Command() {
	case "new":
		cs := b.getOrCreateChat(chatID)
		b.clearPendingMessages(cs)
		b.resetChat(cs)
		b.sendText(chatID, "Session cleared. Send a message to start a new conversation.")

	case "cancel":
		cs := b.getOrCreateChat(chatID)
		b.clearPendingMessages(cs)
		cs.mu.Lock()
		sid := cs.sessionID
		ownerUID := cs.ownerUserID
		cs.mu.Unlock()

		if sid == "" {
			b.sendText(chatID, "No active session.")
			return
		}

		if err := b.agentAPI.CancelSession(ctx, sid, ownerUID); err != nil {
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

func (b *Bot) enqueueIncomingMessage(ctx context.Context, cs *chatState, chatID int64, text string) {
	if text == "" {
		return
	}

	cs.mu.Lock()
	cs.pendingMessages = append(cs.pendingMessages, text)
	cs.pendingFlushGen++
	gen := cs.pendingFlushGen
	cs.mu.Unlock()

	delay := b.incomingDelay
	if delay <= 0 {
		delay = defaultIncomingBatchDelay
	}
	time.AfterFunc(delay, func() {
		b.flushIncomingMessages(ctx, cs, chatID, gen)
	})
}

func (b *Bot) flushIncomingMessages(ctx context.Context, cs *chatState, chatID int64, gen uint64) {
	cs.mu.Lock()
	if gen != cs.pendingFlushGen || len(cs.pendingMessages) == 0 {
		cs.mu.Unlock()
		return
	}
	text := strings.Join(append([]string(nil), cs.pendingMessages...), "\n\n")
	cs.pendingMessages = nil
	cs.mu.Unlock()

	user := b.userIdentity(chatID)
	if cs.sessionID == "" {
		b.createSession(ctx, cs, chatID, user, text)
		return
	}

	sid, _ := b.prepareSessionForMessage(ctx, cs, user)
	if sid == "" {
		b.createSession(ctx, cs, chatID, user, text)
		return
	}
	b.sendMessage(ctx, cs, chatID, user, text)
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
	ownerUID := cs.ownerUserID
	cs.pendingPromptID = ""
	cs.mu.Unlock()

	if sid == "" {
		callback := tgbotapi.NewCallback(cq.ID, "No active session")
		_, _ = b.botAPI.Send(callback)
		return
	}

	resp := agent.UserPromptResponse{
		PromptID:          promptID,
		SelectedOptionIDs: []string{optionID},
	}

	if err := b.agentAPI.SubmitUserResponse(ctx, sid, ownerUID, resp); err != nil {
		b.logger.Warn("Failed to submit prompt response",
			slog.String("session", sid),
			slog.String("user", ownerUID),
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

	b.setActiveSession(cs, sessionID, user.UserID)
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
	ownerUserID := cs.ownerUserID
	cs.pendingPromptID = ""
	cs.mu.Unlock()

	if sid == "" {
		return
	}

	resp := agent.UserPromptResponse{
		PromptID:         promptID,
		FreeTextResponse: text,
	}

	if err := b.agentAPI.SubmitUserResponse(ctx, sid, ownerUserID, resp); err != nil {
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
			// Session ended — clear the session ID so the next message
			// creates a fresh session instead of reusing a dead one.
			cs.mu.Lock()
			if cs.sessionID == sessionID {
				cs.sessionID = ""
			}
			cs.mu.Unlock()
			return
		}
		b.processStreamResponse(cs, chatID, resp)
	}
}

// processStreamResponse handles a stream response and sends relevant content to Telegram.
func (b *Bot) processStreamResponse(cs *chatState, chatID int64, resp agent.StreamResponse) {
	lastDelivered := b.lastDeliveredSeq(cs)
	maxSeen := lastDelivered
	for _, msg := range resp.Messages {
		if msg.SequenceID > maxSeen {
			maxSeen = msg.SequenceID
		}
		if msg.SequenceID != 0 && msg.SequenceID <= lastDelivered {
			continue
		}
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
	if maxSeen > lastDelivered {
		b.markDelivered(cs, maxSeen)
	}
}

// sendPrompt sends a user prompt with inline keyboard buttons.
func (b *Bot) sendPrompt(cs *chatState, chatID int64, prompt *agent.UserPrompt) {
	cs.mu.Lock()
	cs.pendingPromptID = prompt.PromptID
	cs.mu.Unlock()

	text := prompt.Question
	if prompt.Command != "" {
		text += "\n\nCommand: " + prompt.Command
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
	cs.ownerUserID = ""
	cs.subSessionID = ""
	cs.pendingPromptID = ""
	cs.lastDeliveredSeq = 0
	cs.pendingMessages = nil
	cs.pendingFlushGen++
}

// userIdentity creates a chat-scoped UserIdentity so the entire chat shares one session.
func (b *Bot) userIdentity(chatID int64) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("telegram:%d", chatID),
		Username: "telegram",
		Role:     auth.RoleAdmin,
	}
}

func (b *Bot) setActiveSession(cs *chatState, sessionID, ownerUserID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.sessionID != sessionID {
		cs.lastDeliveredSeq = 0
	}
	cs.sessionID = sessionID
	cs.ownerUserID = ownerUserID
}

func (b *Bot) lastDeliveredSeq(cs *chatState) int64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.lastDeliveredSeq
}

func (b *Bot) markDelivered(cs *chatState, seq int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if seq > cs.lastDeliveredSeq {
		cs.lastDeliveredSeq = seq
	}
}

func (b *Bot) clearPendingMessages(cs *chatState) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.pendingMessages = nil
	cs.pendingFlushGen++
}

func (b *Bot) subscriptionActive(cs *chatState, sessionID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.subSessionID == sessionID && cs.subCancel != nil
}

func (b *Bot) prepareSessionForMessage(ctx context.Context, cs *chatState, user agent.UserIdentity) (string, bool) {
	cs.mu.Lock()
	sid := cs.sessionID
	cs.mu.Unlock()

	if sid == "" {
		return "", false
	}

	newSID, rotated, err := b.agentAPI.CompactSessionIfNeeded(ctx, sid, user)
	if err != nil {
		if err == agent.ErrSessionNotFound {
			b.logger.Warn("Session missing during compaction, resetting chat",
				slog.String("session", sid),
				slog.String("user", user.UserID),
			)
			b.resetChat(cs)
			return "", false
		}
		b.logger.Warn("Failed to compact Telegram session",
			slog.String("session", sid),
			slog.String("user", user.UserID),
			slog.String("error", err.Error()),
		)
		return sid, false
	}
	if !rotated {
		return newSID, false
	}

	b.setActiveSession(cs, newSID, user.UserID)
	b.markSessionSnapshotDelivered(ctx, cs, newSID, user.UserID)
	return newSID, true
}

func (b *Bot) markSessionSnapshotDelivered(ctx context.Context, cs *chatState, sessionID, userID string) {
	detail, err := b.agentAPI.GetSessionDetail(ctx, sessionID, userID)
	if err != nil || detail == nil {
		return
	}

	var maxSeq int64
	for _, msg := range detail.Messages {
		if msg.SequenceID > maxSeq {
			maxSeq = msg.SequenceID
		}
	}
	if maxSeq > 0 {
		b.markDelivered(cs, maxSeq)
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
