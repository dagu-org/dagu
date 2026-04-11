// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package discord provides a Discord bot service that bridges Discord
// channels with the Dagu AI agent, allowing users to interact with the agent
// via Discord messages.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/service/chatbridge"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

// maxDiscordMessageLen is the maximum length for a single Discord message.
const maxDiscordMessageLen = 2000
const defaultIncomingBatchDelay = 750 * time.Millisecond
const defaultTypingRefreshInterval = 7 * time.Second

// AgentService is the subset of the agent API that the Discord bot requires.
type AgentService = chatbridge.AgentService

// discordClientAPI is the subset of the discordgo Session that the bot needs,
// extracted so the implementation can be tested without a live gateway.
type discordClientAPI interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelTyping(channelID string, options ...discordgo.RequestOption) error
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error
}

// Config holds configuration for the Discord bot.
type Config struct {
	Token                 string
	AllowedChannelIDs     []string
	InterestedEventTypes  []string
	SafeMode              bool
	RespondToAll          bool // respond to all channel messages, not just @mentions
	EventService          *eventstore.Service
	NotificationStateFile string
}

// chatState tracks the agent session state for a single Discord channel.
type chatState struct {
	chatbridge.State

	channelID string

	typingMu      sync.Mutex
	typingCancel  context.CancelFunc
	typingDone    chan struct{}
	typingLoopGen uint64
}

// Bot is a Discord bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg                   Config
	agentAPI              AgentService
	session               *discordgo.Session
	client                discordClientAPI
	selfID                string
	chats                 sync.Map // channelID (string) -> *chatState
	allowedChannels       map[string]struct{}
	eventService          *eventstore.Service
	notificationStateFile string
	logger                *slog.Logger
	incomingDelay         time.Duration
	typingDelay           time.Duration
}

// New creates a new Discord bot instance.
func New(cfg Config, agentAPI AgentService, logger *slog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord bot token is required (set DAGU_BOTS_DISCORD_TOKEN)")
	}
	if len(cfg.AllowedChannelIDs) == 0 {
		return nil, fmt.Errorf("at least one allowed channel ID is required (set DAGU_BOTS_DISCORD_ALLOWED_CHANNEL_IDS)")
	}

	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// We need message content intent to read user messages, plus guild and DM
	// messages to actually receive them.
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	allowed := make(map[string]struct{}, len(cfg.AllowedChannelIDs))
	for _, id := range cfg.AllowedChannelIDs {
		allowed[id] = struct{}{}
	}

	return &Bot{
		cfg:                   cfg,
		agentAPI:              agentAPI,
		session:               session,
		client:                session,
		allowedChannels:       allowed,
		eventService:          cfg.EventService,
		notificationStateFile: cfg.NotificationStateFile,
		logger:                logger,
		incomingDelay:         defaultIncomingBatchDelay,
		typingDelay:           defaultTypingRefreshInterval,
	}, nil
}

// Run starts the bot and blocks until the context is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Discord bot starting",
		slog.Int("allowed_channels", len(b.allowedChannels)),
		slog.Bool("respond_to_all", b.cfg.RespondToAll),
	)

	b.session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		b.handleMessageCreate(ctx, m)
	})
	b.session.AddHandler(func(_ *discordgo.Session, i *discordgo.InteractionCreate) {
		b.handleInteractionCreate(ctx, i)
	})
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		b.selfID = r.User.ID
		b.logger.Info("Discord bot ready",
			slog.String("username", r.User.Username),
			slog.String("user_id", r.User.ID),
		)
	})

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord gateway connection: %w", err)
	}
	defer func() {
		if err := b.session.Close(); err != nil {
			b.logger.Warn("Failed to close Discord session", slog.String("error", err.Error()))
		}
	}()

	// Start DAG run monitor if the event store is available.
	if b.eventService != nil {
		monitor := NewDAGRunMonitor(b.eventService, b.notificationStateFile, b.agentAPI, b, b.logger)
		go monitor.Run(ctx)
	} else {
		b.logger.Warn("Event store is not configured; DAG run notifications are disabled")
	}

	<-ctx.Done()
	b.logger.Info("Discord bot stopped")
	return nil
}

// handleMessageCreate processes an incoming Discord message.
func (b *Bot) handleMessageCreate(ctx context.Context, m *discordgo.MessageCreate) {
	if m.Author == nil {
		return
	}
	// Ignore our own messages and messages from other bots.
	if m.Author.Bot || (b.selfID != "" && m.Author.ID == b.selfID) {
		return
	}

	channelID := m.ChannelID
	if channelID == "" {
		return
	}

	// DMs are always handled regardless of the allow list, so long as the
	// token owner opened the DM.
	isDM := m.GuildID == ""
	if !isDM && !b.isChannelAllowed(channelID) {
		return
	}

	text := strings.TrimSpace(m.Content)
	if text == "" {
		return
	}

	// @mention handling: strip "<@selfID>" / "<@!selfID>" prefixes when present.
	wasMentioned := false
	if b.selfID != "" {
		mentions := []string{"<@" + b.selfID + ">", "<@!" + b.selfID + ">"}
		for _, prefix := range mentions {
			if trimmed, ok := strings.CutPrefix(text, prefix); ok {
				text = strings.TrimSpace(trimmed)
				wasMentioned = true
				break
			}
		}
		if !wasMentioned {
			for _, u := range m.Mentions {
				if u.ID == b.selfID {
					wasMentioned = true
					break
				}
			}
		}
	}

	if text == "" {
		return
	}

	// In guild channels, when respond_to_all is disabled, only handle
	// messages that @mention the bot.
	if !isDM && !b.cfg.RespondToAll && !wasMentioned {
		return
	}

	cs := b.getOrCreateChat(channelID)

	// Handle text commands (works in both DMs and guild channels).
	if fields := strings.Fields(text); len(fields) > 0 {
		if cmd := fields[0]; cmd == "new" || cmd == "cancel" {
			b.clearPendingMessages(cs)
			b.handleTextCommand(ctx, cs, channelID, cmd)
			return
		}
	}

	pendingPrompt := cs.PendingPromptID()
	if pendingPrompt != "" {
		b.submitPromptResponse(ctx, cs, channelID, pendingPrompt, text)
		return
	}

	b.enqueueIncomingMessage(ctx, cs, channelID, text)
}

// handleInteractionCreate handles button interactions.
func (b *Bot) handleInteractionCreate(ctx context.Context, i *discordgo.InteractionCreate) {
	if i.Interaction == nil || i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()

	// Parse custom_id: "prompt:<promptID>:<optionID>"
	parts := strings.SplitN(data.CustomID, ":", 3)
	if len(parts) != 3 || parts[0] != "prompt" {
		return
	}

	promptID := parts[1]
	optionID := parts[2]

	channelID := i.ChannelID
	cs := b.getOrCreateChat(channelID)
	sid, ownerUID := cs.ActiveSession()
	cs.ClearPendingPrompt()

	if sid == "" {
		b.ackInteraction(i.Interaction)
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
		b.ackInteraction(i.Interaction)
		b.sendText(channelID, fmt.Sprintf("Failed to submit response: %s", err.Error()))
		return
	}

	// Update the original message in-place to show the selection and remove
	// the now-consumed buttons.
	updatedContent := ""
	if i.Message != nil {
		updatedContent = i.Message.Content + "\n\nSelected: " + optionID
	} else {
		updatedContent = "Selected: " + optionID
	}

	emptyComponents := []discordgo.MessageComponent{}
	err := b.client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    updatedContent,
			Components: emptyComponents,
		},
	})
	if err != nil {
		b.logger.Warn("Failed to update interaction message", slog.String("error", err.Error()))
	}
}

func (b *Bot) ackInteraction(interaction *discordgo.Interaction) {
	err := b.client.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		b.logger.Debug("Failed to ack interaction", slog.String("error", err.Error()))
	}
}

// handleTextCommand processes text commands ("new", "cancel").
func (b *Bot) handleTextCommand(ctx context.Context, cs *chatState, channelID, cmd string) {
	switch cmd {
	case "new":
		b.resetChat(cs)
		b.sendText(channelID, "Session cleared. Send a message to start a new conversation.")

	case "cancel":
		b.stopTypingLoop(cs)
		sid, ownerUID := cs.ActiveSession()

		if sid == "" {
			b.sendText(channelID, "No active session.")
			return
		}

		if err := b.agentAPI.CancelSession(ctx, sid, ownerUID); err != nil {
			b.sendText(channelID, "Failed to cancel session: "+err.Error())
			return
		}
		b.sendText(channelID, "Session cancelled.")
	}
}

func (b *Bot) enqueueIncomingMessage(ctx context.Context, cs *chatState, channelID, text string) {
	if text == "" {
		return
	}

	b.startTypingLoop(ctx, cs, channelID)
	gen := cs.EnqueuePendingMessage(text)

	delay := b.incomingDelay
	if delay <= 0 {
		delay = defaultIncomingBatchDelay
	}
	time.AfterFunc(delay, func() {
		b.flushIncomingMessages(ctx, cs, channelID, gen)
	})
}

func (b *Bot) flushIncomingMessages(ctx context.Context, cs *chatState, channelID string, gen uint64) {
	text, ok := cs.TakePendingMessages(gen, "\n\n")
	if !ok {
		return
	}

	user := b.userIdentity(channelID)
	if cs.SessionID() == "" {
		b.createSession(ctx, cs, channelID, user, text)
		return
	}

	b.sendAgentMessage(ctx, cs, channelID, user, text)
}

// ---------------------------------------------------------------------------
// Agent session management
// ---------------------------------------------------------------------------

// createSession creates a new agent session and starts listening for responses.
func (b *Bot) createSession(ctx context.Context, cs *chatState, channelID string, user agent.UserIdentity, text string) {
	b.startTypingLoop(ctx, cs, channelID)

	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	sessionID, _, err := b.agentAPI.CreateSession(ctx, user, req)
	if err != nil {
		b.stopTypingLoop(cs)
		b.logger.Error("Failed to create session", slog.String("error", err.Error()))
		b.sendText(channelID, "Failed to start session: "+err.Error())
		return
	}

	b.setActiveSession(cs, sessionID, user.UserID)
	b.startSubscription(ctx, cs, channelID, user.UserID, sessionID)
}

// sendAgentMessage sends a message to an existing session.
func (b *Bot) sendAgentMessage(ctx context.Context, cs *chatState, channelID string, user agent.UserIdentity, text string) {
	b.startTypingLoop(ctx, cs, channelID)

	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	result, err := chatbridge.EnqueueMessage(ctx, b.agentAPI, &cs.State, user, req)
	if err != nil {
		b.stopTypingLoop(cs)
		b.logger.Error("Failed to enqueue message", slog.String("error", err.Error()))
		b.sendText(channelID, "Failed to send message: "+err.Error())
		return
	}
	if result.Missing {
		b.logger.Warn("Session missing during Discord send, recreating chat",
			slog.String("session", result.SessionID),
			slog.String("user", user.UserID),
		)
		b.resetChat(cs)
		b.createSession(ctx, cs, channelID, user, text)
		return
	}
	sid := result.SessionID
	if sid == "" {
		sid = cs.SessionID()
	}
	if result.Queued {
		b.startTypingLoop(ctx, cs, channelID)
	}
	if sid != "" {
		b.ensureSubscription(ctx, cs, channelID, user.UserID, sid)
	}
}

// submitPromptResponse submits a text response to a pending agent prompt.
func (b *Bot) submitPromptResponse(ctx context.Context, cs *chatState, channelID, promptID, text string) {
	sid, ownerUserID := cs.ActiveSession()
	cs.ClearPendingPrompt()

	if sid == "" {
		return
	}

	resp := agent.UserPromptResponse{
		PromptID:         promptID,
		FreeTextResponse: text,
	}

	b.startTypingLoop(ctx, cs, channelID)
	if err := b.agentAPI.SubmitUserResponse(ctx, sid, ownerUserID, resp); err != nil {
		b.stopTypingLoop(cs)
		b.sendText(channelID, "Failed to submit response: "+err.Error())
	}
}

// ---------------------------------------------------------------------------
// Subscription management
// ---------------------------------------------------------------------------

// ensureSubscription starts a subscription only if one isn't already running
// for the given session.
func (b *Bot) ensureSubscription(ctx context.Context, cs *chatState, channelID, userID, sessionID string) {
	subCtx, cancel := context.WithCancel(ctx)
	cleanup, started := cs.PrepareSubscription(sessionID, cancel, false)
	if !started {
		if cleanup != nil {
			cleanup()
		}
		return
	}
	if cleanup != nil {
		cleanup()
	}

	go b.subscribeLoop(subCtx, cs, channelID, userID, sessionID)
}

// startSubscription cancels any existing subscription and starts a fresh one.
func (b *Bot) startSubscription(ctx context.Context, cs *chatState, channelID, userID, sessionID string) {
	subCtx, cancel := context.WithCancel(ctx)
	cleanup, _ := cs.PrepareSubscription(sessionID, cancel, true)
	if cleanup != nil {
		cleanup()
	}

	go b.subscribeLoop(subCtx, cs, channelID, userID, sessionID)
}

// subscribeLoop uses the agent's built-in pub-sub to receive session updates
// in real time and forward them to Discord.
func (b *Bot) subscribeLoop(ctx context.Context, cs *chatState, channelID, userID, sessionID string) {
	user := agent.UserIdentity{
		UserID:   userID,
		Username: "discord",
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

	b.processStreamResponse(ctx, cs, channelID, snapshot)

	for {
		resp, ok := next()
		if !ok {
			b.stopTypingLoop(cs)
			// Session ended — clear the session ID so the next message
			// creates a fresh session instead of reusing a dead one.
			cs.ClearSessionIfActive(sessionID)
			return
		}
		b.processStreamResponse(ctx, cs, channelID, resp)
	}
}

// processStreamResponse handles a stream response and sends relevant content to Discord.
func (b *Bot) processStreamResponse(ctx context.Context, cs *chatState, channelID string, resp agent.StreamResponse) {
	var shouldFlushQueued bool
	chatbridge.ProcessStreamResponse(&cs.State, resp, chatbridge.StreamHandlers{
		OnSessionState: func(state agent.SessionState) {
			if !state.Working && state.HasQueuedUserInput {
				shouldFlushQueued = true
				b.startTypingLoop(ctx, cs, channelID)
			}
		},
		OnWorking: func(working bool) {
			if working {
				b.startTypingLoop(ctx, cs, channelID)
			} else if !cs.HasQueuedUserInput() {
				b.stopTypingLoop(cs)
			}
		},
		OnAssistant: func(msg agent.Message) {
			b.stopTypingLoop(cs)
			b.sendLongText(channelID, msg.Content)
		},
		OnError: func(msg agent.Message) {
			b.stopTypingLoop(cs)
			b.sendText(channelID, "Error: "+msg.Content)
		},
		OnPrompt: func(prompt *agent.UserPrompt) {
			b.stopTypingLoop(cs)
			b.sendPrompt(cs, channelID, prompt)
		},
	})
	if shouldFlushQueued {
		_, ownerUserID := cs.ActiveSession()
		if ownerUserID == "" {
			return
		}
		user := agent.UserIdentity{
			UserID:   ownerUserID,
			Username: "discord",
			Role:     auth.RoleAdmin,
		}
		result, err := chatbridge.FlushQueuedMessage(ctx, b.agentAPI, &cs.State, user)
		if err != nil {
			b.logger.Warn("Failed to flush queued Discord message",
				slog.String("session", result.SessionID),
				slog.String("user", user.UserID),
				slog.String("error", err.Error()),
			)
			return
		}
		if result.Missing {
			b.logger.Warn("Session missing during queued Discord flush",
				slog.String("session", result.SessionID),
				slog.String("user", user.UserID),
			)
			b.resetChat(cs)
			return
		}
		if result.SessionID != "" {
			b.ensureSubscription(ctx, cs, channelID, user.UserID, result.SessionID)
		}
	}
}

// ---------------------------------------------------------------------------
// Discord messaging helpers
// ---------------------------------------------------------------------------

// sendPrompt sends a user prompt with interactive buttons.
func (b *Bot) sendPrompt(cs *chatState, channelID string, prompt *agent.UserPrompt) {
	cs.SetPendingPrompt(prompt.PromptID)

	text := prompt.Question
	if prompt.Command != "" {
		text += "\n\nCommand: " + prompt.Command
	}
	if prompt.AllowFreeText {
		text += "\n\n(You can also reply with text)"
	}

	if len(prompt.Options) == 0 {
		b.sendText(channelID, text)
		return
	}

	// Discord allows up to 5 components per action row and up to 5 rows per message.
	var rows []discordgo.MessageComponent
	var current []discordgo.MessageComponent
	for _, opt := range prompt.Options {
		customID := fmt.Sprintf("prompt:%s:%s", prompt.PromptID, opt.ID)
		label := opt.Label
		if opt.Description != "" {
			label += " - " + opt.Description
		}
		// Discord button label limit is 80 characters.
		if len(label) > 80 {
			label = label[:77] + "..."
		}
		current = append(current, discordgo.Button{
			Label:    label,
			Style:    discordgo.PrimaryButton,
			CustomID: customID,
		})
		if len(current) == 5 {
			rows = append(rows, discordgo.ActionsRow{Components: current})
			current = nil
			if len(rows) == 5 {
				break
			}
		}
	}
	if len(current) > 0 && len(rows) < 5 {
		rows = append(rows, discordgo.ActionsRow{Components: current})
	}

	// If the message body is too long for Discord, split it and only attach
	// buttons to the final chunk.
	chunks := splitMessage(text, maxDiscordMessageLen)
	for i, chunk := range chunks {
		if i < len(chunks)-1 {
			b.sendText(channelID, chunk)
			continue
		}
		if _, err := b.client.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content:    chunk,
			Components: rows,
		}); err != nil {
			b.logger.Warn("Failed to send prompt",
				slog.String("channel_id", channelID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// sendLongText sends a message, splitting it if it exceeds Discord's limit.
func (b *Bot) sendLongText(channelID, text string) {
	chunks := splitMessage(text, maxDiscordMessageLen)
	for _, chunk := range chunks {
		b.sendText(channelID, chunk)
	}
}

// sendText sends a simple text message to a Discord channel.
func (b *Bot) sendText(channelID, text string) {
	if text == "" {
		return
	}
	if _, err := b.client.ChannelMessageSend(channelID, text); err != nil {
		b.logger.Warn("Failed to send Discord message",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()),
		)
	}
}

// ---------------------------------------------------------------------------
// Typing indicator management
// ---------------------------------------------------------------------------

// sendTyping sends a "typing..." indicator to the Discord channel.
func (b *Bot) sendTyping(channelID string) {
	if err := b.client.ChannelTyping(channelID); err != nil {
		b.logger.Debug("Failed to send typing indicator", slog.String("error", err.Error()))
	}
}

func (b *Bot) startTypingLoop(ctx context.Context, cs *chatState, channelID string) {
	cs.typingMu.Lock()
	if cs.typingCancel != nil {
		cs.typingMu.Unlock()
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	cs.typingLoopGen++
	gen := cs.typingLoopGen
	cs.typingCancel = cancel
	cs.typingDone = done
	cs.typingMu.Unlock()

	if !b.sendTypingIfCurrent(cs, gen, channelID) {
		close(done)
		b.finishTypingLoop(cs, gen)
		return
	}

	refresh := b.typingDelay
	if refresh <= 0 {
		refresh = defaultTypingRefreshInterval
	}

	go func() {
		ticker := time.NewTicker(refresh)
		defer ticker.Stop()
		defer close(done)
		defer b.finishTypingLoop(cs, gen)

		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				select {
				case <-loopCtx.Done():
					return
				default:
				}
				if !b.sendTypingIfCurrent(cs, gen, channelID) {
					return
				}
			}
		}
	}()
}

func (b *Bot) stopTypingLoop(cs *chatState) {
	cs.typingMu.Lock()
	cancel := cs.typingCancel
	done := cs.typingDone
	cs.typingCancel = nil
	cs.typingDone = nil
	cs.typingMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (b *Bot) finishTypingLoop(cs *chatState, gen uint64) {
	cs.typingMu.Lock()
	defer cs.typingMu.Unlock()
	if cs.typingLoopGen == gen {
		cs.typingCancel = nil
		cs.typingDone = nil
	}
}

func (b *Bot) sendTypingIfCurrent(cs *chatState, gen uint64, channelID string) bool {
	cs.typingMu.Lock()
	defer cs.typingMu.Unlock()
	if cs.typingLoopGen != gen || cs.typingCancel == nil {
		return false
	}
	b.sendTyping(channelID)
	return true
}

// ---------------------------------------------------------------------------
// State management
// ---------------------------------------------------------------------------

// isChannelAllowed checks if a channel ID is authorized to use the bot.
func (b *Bot) isChannelAllowed(channelID string) bool {
	_, ok := b.allowedChannels[channelID]
	return ok
}

// getOrCreateChat returns or creates a chatState for the given channel ID.
func (b *Bot) getOrCreateChat(channelID string) *chatState {
	val, _ := b.chats.LoadOrStore(channelID, &chatState{channelID: channelID})
	return val.(*chatState)
}

// resetChat clears the session state for a chat.
func (b *Bot) resetChat(cs *chatState) {
	if cancel := cs.Reset(); cancel != nil {
		cancel()
	}
	b.stopTypingLoop(cs)
}

// userIdentity creates a channel-scoped UserIdentity so the entire channel shares one session.
func (b *Bot) userIdentity(channelID string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("discord:%s", channelID),
		Username: "discord",
		Role:     auth.RoleAdmin,
	}
}

func (b *Bot) setActiveSession(cs *chatState, sessionID, ownerUserID string) {
	cs.SetActiveSession(sessionID, ownerUserID)
}

func (b *Bot) markDelivered(cs *chatState, seq int64) {
	cs.MarkDelivered(seq)
}

func (b *Bot) clearPendingMessages(cs *chatState) {
	cs.ClearPendingMessages()
}

func (b *Bot) subscriptionActive(cs *chatState, sessionID string) bool {
	return cs.SubscriptionActive(sessionID)
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// splitMessage splits text into chunks that fit within the Discord message limit.
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
