// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package slack provides a Slack bot service that bridges Slack
// channels with the Dagu AI agent, allowing users to interact with the agent
// via Slack messages using Socket Mode.
package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// maxSlackMessageLen is the maximum length for a single Slack message.
const maxSlackMessageLen = 4000

// defaultContextLimit is the assumed context window size when no model config is available.
const defaultContextLimit = 200_000

// contextRotationRatio is the fraction of the context limit at which sessions are rotated.
const contextRotationRatio = 0.5

// AgentService is the subset of the agent API that the Slack bot requires.
type AgentService interface {
	CreateSession(ctx context.Context, user agent.UserIdentity, req agent.ChatRequest) (string, string, error)
	SendMessage(ctx context.Context, sessionID string, user agent.UserIdentity, req agent.ChatRequest) error
	CancelSession(ctx context.Context, sessionID, userID string) error
	SubmitUserResponse(ctx context.Context, sessionID, userID string, resp agent.UserPromptResponse) error
	GetSessionDetail(ctx context.Context, sessionID, userID string) (*agent.StreamResponse, error)
	SubscribeSession(ctx context.Context, sessionID string, user agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error)
}

// Config holds configuration for the Slack bot.
type Config struct {
	BotToken          string
	AppToken          string
	AllowedChannelIDs []string
	SafeMode          bool
	RespondToAll      bool             // respond to all channel messages, not just @mentions
	DAGRunStore       exec.DAGRunStore // optional: enables DAG run monitoring
}

// messageRef identifies a specific Slack message for reaction management.
type messageRef struct {
	channel   string
	timestamp string
}

// chatState tracks the agent session state for a single conversation.
// A conversation is either a DM channel or a specific thread in a channel.
type chatState struct {
	channelID       string // Slack channel ID
	threadTS        string // thread parent timestamp (empty for DMs)
	sessionID       string
	ownerUserID     string // user ID that owns the session
	subSessionID    string // session ID the subscription is listening to
	subCancel       context.CancelFunc
	mu              sync.Mutex
	pendingPromptID  string
	pendingReaction  *messageRef // message awaiting reaction removal on first response
	thinkingMessage  *messageRef // "Thinking..." message to delete on first response
}

// Bot is a Slack bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg             Config
	agentAPI        AgentService
	slackClient     *slack.Client
	socketClient    *socketmode.Client
	chats           sync.Map // conversationKey -> *chatState
	activeThreads   sync.Map // "channelID:threadTS" -> true
	allowedChannels map[string]struct{}
	dagRunStore     exec.DAGRunStore
	logger          *slog.Logger
}

// New creates a new Slack bot instance.
func New(cfg Config, agentAPI AgentService, logger *slog.Logger) (*Bot, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("slack bot token is required (set DAGU_BOTS_SLACK_BOT_TOKEN)")
	}
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("slack app-level token is required (set DAGU_BOTS_SLACK_APP_TOKEN)")
	}
	if len(cfg.AllowedChannelIDs) == 0 {
		return nil, fmt.Errorf("at least one allowed channel ID is required (set DAGU_BOTS_SLACK_ALLOWED_CHANNEL_IDS)")
	}

	slackClient := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	socketClient := socketmode.New(
		slackClient,
		socketmode.OptionLog(slog.NewLogLogger(logger.Handler(), slog.LevelDebug)),
	)

	allowed := make(map[string]struct{}, len(cfg.AllowedChannelIDs))
	for _, id := range cfg.AllowedChannelIDs {
		allowed[id] = struct{}{}
	}

	return &Bot{
		cfg:             cfg,
		agentAPI:        agentAPI,
		slackClient:     slackClient,
		socketClient:    socketClient,
		allowedChannels: allowed,
		dagRunStore:     cfg.DAGRunStore,
		logger:          logger,
	}, nil
}

// Run starts the bot and blocks until the context is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Slack bot started",
		slog.Int("allowed_channels", len(b.allowedChannels)),
	)

	// Start DAG run monitor if a DAGRunStore is available
	if b.dagRunStore != nil {
		monitor := NewDAGRunMonitor(b.dagRunStore, b.agentAPI, b, b.logger)
		go monitor.Run(ctx)
	}

	// Start the socket mode client in a goroutine
	go func() {
		if err := b.socketClient.RunContext(ctx); err != nil {
			b.logger.Error("Socket mode client error", slog.String("error", err.Error()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Slack bot stopped")
			return nil

		case evt, ok := <-b.socketClient.Events:
			if !ok {
				return nil
			}
			b.handleEvent(ctx, evt)
		}
	}
}

// handleEvent routes a socket mode event to the appropriate handler.
func (b *Bot) handleEvent(ctx context.Context, evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		b.socketClient.Ack(*evt.Request)
		b.handleEventsAPI(ctx, eventsAPIEvent)

	case socketmode.EventTypeInteractive:
		callback, ok := evt.Data.(slack.InteractionCallback)
		if !ok {
			return
		}
		b.socketClient.Ack(*evt.Request)
		b.handleInteraction(ctx, callback)

	case socketmode.EventTypeSlashCommand:
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		b.socketClient.Ack(*evt.Request)
		b.handleSlashCommand(ctx, cmd)
	}
}

// handleEventsAPI processes Events API events (messages, app mentions).
func (b *Bot) handleEventsAPI(ctx context.Context, evt slackevents.EventsAPIEvent) {
	switch slackevents.EventsAPIType(evt.InnerEvent.Type) {
	case slackevents.AppMention:
		ev, ok := evt.InnerEvent.Data.(*slackevents.AppMentionEvent)
		if !ok {
			return
		}
		text := stripBotMention(ev.Text)
		if text == "" {
			return
		}

		// Determine thread TS: if already in a thread, use it;
		// otherwise start a new thread from this message.
		threadTS := ev.ThreadTimeStamp
		if threadTS == "" {
			threadTS = ev.TimeStamp
		}

		b.handleChannelMessage(ctx, ev.Channel, ev.User, ev.TimeStamp, threadTS, text)

	case slackevents.Message:
		ev, ok := evt.InnerEvent.Data.(*slackevents.MessageEvent)
		if !ok {
			return
		}
		// Only handle regular messages (not bot messages, edits, etc.)
		if ev.SubType != "" || ev.BotID != "" {
			return
		}

		// DMs: always respond.
		if ev.ChannelType == "im" {
			b.handleDMMessage(ctx, ev.Channel, ev.User, ev.TimeStamp, ev.Text)
			return
		}

		// Channel thread reply: respond without @mention if bot is
		// already participating in this thread.
		if ev.ThreadTimeStamp != "" {
			threadKey := ev.Channel + ":" + ev.ThreadTimeStamp
			if _, ok := b.activeThreads.Load(threadKey); ok {
				b.handleChannelMessage(ctx, ev.Channel, ev.User, ev.TimeStamp, ev.ThreadTimeStamp, ev.Text)
				return
			}
		}

		// respond_to_all: treat every channel message as a conversation.
		if b.cfg.RespondToAll {
			b.handleDMMessage(ctx, ev.Channel, ev.User, ev.TimeStamp, ev.Text)
		}
	}
}

// handleChannelMessage processes a message in a channel (from @mention or thread reply).
func (b *Bot) handleChannelMessage(ctx context.Context, channelID, userID, msgTimestamp, threadTS, text string) {
	if !b.isChannelAllowed(channelID) {
		return
	}

	// Track this thread so future replies don't need @mention
	threadKey := channelID + ":" + threadTS
	b.activeThreads.Store(threadKey, true)

	cs := b.getOrCreateChat(threadKey, channelID, threadTS)
	b.processIncoming(ctx, cs, threadKey, userID, msgTimestamp, text)
}

// handleDMMessage processes a direct message.
func (b *Bot) handleDMMessage(ctx context.Context, channelID, userID, msgTimestamp, text string) {
	cs := b.getOrCreateChat(channelID, channelID, "")
	b.processIncoming(ctx, cs, channelID, userID, msgTimestamp, text)
}

// processIncoming is the core message handler shared by channel and DM flows.
// convKey uniquely identifies the conversation (channelID or channelID:threadTS).
func (b *Bot) processIncoming(ctx context.Context, cs *chatState, convKey, userID, msgTimestamp, text string) {
	if text == "" {
		return
	}

	// Handle text commands
	if cmd := strings.Fields(text)[0]; cmd == "new" || cmd == "cancel" {
		b.handleTextCommand(ctx, cs, cmd)
		return
	}

	// Check if this is a response to a pending prompt
	cs.mu.Lock()
	pendingPrompt := cs.pendingPromptID
	cs.mu.Unlock()

	if pendingPrompt != "" {
		b.submitPromptResponse(ctx, cs, pendingPrompt, text)
		return
	}

	// Add a "thinking" reaction and post a typing indicator message
	b.addReaction(cs.channelID, msgTimestamp, "hourglass_flowing_sand")
	thinkingTS := b.postThinking(cs)

	user := b.userIdentity(convKey, userID)

	cs.mu.Lock()
	cs.pendingReaction = &messageRef{channel: cs.channelID, timestamp: msgTimestamp}
	if thinkingTS != "" {
		cs.thinkingMessage = &messageRef{channel: cs.channelID, timestamp: thinkingTS}
	}
	cs.mu.Unlock()

	if cs.sessionID == "" {
		b.createSession(ctx, cs, user, text)
	} else {
		if b.shouldRotateSession(ctx, cs, user.UserID) {
			b.rotateSession(ctx, cs, user, text)
		} else {
			b.sendAgentMessage(ctx, cs, user, text)
		}
	}
}

// handleTextCommand processes text commands ("new", "cancel").
func (b *Bot) handleTextCommand(ctx context.Context, cs *chatState, cmd string) {
	switch cmd {
	case "new":
		b.resetChat(cs)
		b.sendReply(cs, "Session cleared. Send a message to start a new conversation.")

	case "cancel":
		cs.mu.Lock()
		sid := cs.sessionID
		ownerUID := cs.ownerUserID
		cs.mu.Unlock()

		if sid == "" {
			b.sendReply(cs, "No active session.")
			return
		}

		if err := b.agentAPI.CancelSession(ctx, sid, ownerUID); err != nil {
			b.sendReply(cs, "Failed to cancel session: "+err.Error())
			return
		}
		b.sendReply(cs, "Session cancelled.")
	}
}

// handleSlashCommand processes Slack slash commands.
func (b *Bot) handleSlashCommand(ctx context.Context, cmd slack.SlashCommand) {
	channelID := cmd.ChannelID

	if !b.isChannelAllowed(channelID) {
		return
	}

	// Slash commands don't have thread context, so use channel-level state.
	cs := b.getOrCreateChat(channelID, channelID, "")

	switch cmd.Command {
	case "/dagu-new":
		b.resetChat(cs)
		b.sendReply(cs, "Session cleared. Send a message to start a new conversation.")

	case "/dagu-cancel":
		cs.mu.Lock()
		sid := cs.sessionID
		ownerUID := cs.ownerUserID
		cs.mu.Unlock()

		if sid == "" {
			b.sendReply(cs, "No active session.")
			return
		}

		if err := b.agentAPI.CancelSession(ctx, sid, ownerUID); err != nil {
			b.sendReply(cs, "Failed to cancel session: "+err.Error())
			return
		}
		b.sendReply(cs, "Session cancelled.")

	default:
		b.sendReply(cs, "Unknown command. Use /dagu-new, /dagu-cancel, or just send a message.")
	}
}

// handleInteraction processes interactive component callbacks (button presses).
func (b *Bot) handleInteraction(ctx context.Context, callback slack.InteractionCallback) {
	channelID := callback.Channel.ID

	if len(callback.ActionCallback.BlockActions) == 0 {
		return
	}

	action := callback.ActionCallback.BlockActions[0]

	// Parse action ID: "prompt:<promptID>:<optionID>"
	parts := strings.SplitN(action.ActionID, ":", 3)
	if len(parts) != 3 || parts[0] != "prompt" {
		return
	}

	promptID := parts[1]
	optionID := parts[2]

	// Determine conversation key from the message thread
	threadTS := callback.Message.ThreadTimestamp
	convKey := channelID
	if threadTS != "" {
		convKey = channelID + ":" + threadTS
	}

	cs := b.getOrCreateChat(convKey, channelID, threadTS)
	cs.mu.Lock()
	sid := cs.sessionID
	ownerUID := cs.ownerUserID
	cs.pendingPromptID = ""
	cs.mu.Unlock()

	if sid == "" {
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
		b.sendReply(cs, fmt.Sprintf("Failed to submit response: %s", err.Error()))
		return
	}

	// Update the original message to show the selection
	_, _, _, err := b.slackClient.SendMessage(
		channelID,
		slack.MsgOptionUpdate(callback.Message.Timestamp),
		slack.MsgOptionText(callback.Message.Text+"\n\nSelected: "+optionID, false),
	)
	if err != nil {
		b.logger.Warn("Failed to update message", slog.String("error", err.Error()))
	}
}

// ---------------------------------------------------------------------------
// Agent session management
// ---------------------------------------------------------------------------

// createSession creates a new agent session and starts listening for responses.
func (b *Bot) createSession(ctx context.Context, cs *chatState, user agent.UserIdentity, text string) {
	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	sessionID, _, err := b.agentAPI.CreateSession(ctx, user, req)
	if err != nil {
		b.logger.Error("Failed to create session", slog.String("error", err.Error()))
		b.sendReply(cs, "Failed to start session: "+err.Error())
		return
	}

	cs.mu.Lock()
	cs.sessionID = sessionID
	cs.ownerUserID = user.UserID
	cs.mu.Unlock()

	b.startSubscription(ctx, cs, user.UserID, sessionID)
}

// sendAgentMessage sends a message to an existing session.
func (b *Bot) sendAgentMessage(ctx context.Context, cs *chatState, user agent.UserIdentity, text string) {
	cs.mu.Lock()
	sid := cs.sessionID
	cs.mu.Unlock()

	req := agent.ChatRequest{
		Message:  text,
		SafeMode: b.cfg.SafeMode,
	}

	if err := b.agentAPI.SendMessage(ctx, sid, user, req); err != nil {
		b.logger.Error("Failed to send message", slog.String("error", err.Error()))
		b.sendReply(cs, "Failed to send message: "+err.Error())
		return
	}

	b.ensureSubscription(ctx, cs, user.UserID, sid)
}

// submitPromptResponse submits a text response to a pending agent prompt.
func (b *Bot) submitPromptResponse(ctx context.Context, cs *chatState, promptID, text string) {
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
		b.sendReply(cs, "Failed to submit response: "+err.Error())
	}
}

// ---------------------------------------------------------------------------
// Subscription management
// ---------------------------------------------------------------------------

// ensureSubscription starts a subscription only if one isn't already running
// for the given session.
func (b *Bot) ensureSubscription(ctx context.Context, cs *chatState, userID, sessionID string) {
	cs.mu.Lock()
	if cs.subSessionID == sessionID && cs.subCancel != nil {
		cs.mu.Unlock()
		return
	}
	if cs.subCancel != nil {
		cs.subCancel()
	}
	subCtx, cancel := context.WithCancel(ctx)
	cs.subCancel = cancel
	cs.subSessionID = sessionID
	cs.mu.Unlock()

	go b.subscribeLoop(subCtx, cs, userID, sessionID)
}

// startSubscription cancels any existing subscription and starts a fresh one.
func (b *Bot) startSubscription(ctx context.Context, cs *chatState, userID, sessionID string) {
	cs.mu.Lock()
	if cs.subCancel != nil {
		cs.subCancel()
	}
	subCtx, cancel := context.WithCancel(ctx)
	cs.subCancel = cancel
	cs.subSessionID = sessionID
	cs.mu.Unlock()

	go b.subscribeLoop(subCtx, cs, userID, sessionID)
}

// subscribeLoop uses the agent's built-in pub-sub to receive session updates
// in real time and forward them to Slack.
func (b *Bot) subscribeLoop(ctx context.Context, cs *chatState, userID, sessionID string) {
	user := agent.UserIdentity{
		UserID:   userID,
		Username: "slack",
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

	b.processStreamResponse(cs, snapshot)

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
		b.processStreamResponse(cs, resp)
	}
}

// processStreamResponse handles a stream response and sends relevant content to Slack.
func (b *Bot) processStreamResponse(cs *chatState, resp agent.StreamResponse) {
	for _, msg := range resp.Messages {
		switch msg.Type {
		case agent.MessageTypeAssistant:
			if msg.Content != "" {
				b.clearPendingIndicators(cs)
				b.sendLongReply(cs, msg.Content)
			}

		case agent.MessageTypeError:
			if msg.Content != "" {
				b.clearPendingIndicators(cs)
				b.sendReply(cs, "Error: "+msg.Content)
			}

		case agent.MessageTypeUserPrompt:
			if msg.UserPrompt != nil {
				b.clearPendingIndicators(cs)
				b.sendPrompt(cs, msg.UserPrompt)
			}

		case agent.MessageTypeUser, agent.MessageTypeUIAction:
			// Skip user messages and UI actions in Slack output
		}
	}
}

// ---------------------------------------------------------------------------
// Slack messaging helpers
// ---------------------------------------------------------------------------

// sendPrompt sends a user prompt with interactive buttons.
func (b *Bot) sendPrompt(cs *chatState, prompt *agent.UserPrompt) {
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
		var buttons []slack.BlockElement
		for _, opt := range prompt.Options {
			actionID := fmt.Sprintf("prompt:%s:%s", prompt.PromptID, opt.ID)
			label := opt.Label
			if opt.Description != "" {
				label += " - " + opt.Description
			}
			// Truncate label to Slack's 75-char limit for button text
			if len(label) > 75 {
				label = label[:72] + "..."
			}
			buttons = append(buttons, slack.NewButtonBlockElement(
				actionID,
				opt.ID,
				slack.NewTextBlockObject(slack.PlainTextType, label, false, false),
			))
		}

		blocks := []slack.Block{
			slack.NewSectionBlock(
				slack.NewTextBlockObject(slack.MarkdownType, text, false, false),
				nil, nil,
			),
			slack.NewActionBlock("prompt_actions", buttons...),
		}

		opts := []slack.MsgOption{slack.MsgOptionBlocks(blocks...)}
		if cs.threadTS != "" {
			opts = append(opts, slack.MsgOptionTS(cs.threadTS))
		}

		_, _, err := b.slackClient.PostMessage(cs.channelID, opts...)
		if err != nil {
			b.logger.Warn("Failed to send prompt", slog.String("error", err.Error()))
		}
	} else {
		b.sendReply(cs, text)
	}
}

// sendLongReply sends a message, splitting it if it exceeds Slack's limit.
func (b *Bot) sendLongReply(cs *chatState, text string) {
	chunks := splitMessage(text, maxSlackMessageLen)
	for _, chunk := range chunks {
		b.sendReply(cs, chunk)
	}
}

// sendReply sends a text message to the correct channel and thread.
func (b *Bot) sendReply(cs *chatState, text string) {
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if cs.threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(cs.threadTS))
	}

	_, _, err := b.slackClient.PostMessage(cs.channelID, opts...)
	if err != nil {
		b.logger.Warn("Failed to send Slack message",
			slog.String("channel_id", cs.channelID),
			slog.String("error", err.Error()),
		)
	}
}

// sendText sends a simple text message to a channel (used by monitor for notifications).
func (b *Bot) sendText(channelID, text string) {
	_, _, err := b.slackClient.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		b.logger.Warn("Failed to send Slack message",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()),
		)
	}
}

// sendLongText sends a long message to a channel, splitting if needed (used by monitor).
func (b *Bot) sendLongText(channelID, text string) {
	chunks := splitMessage(text, maxSlackMessageLen)
	for _, chunk := range chunks {
		b.sendText(channelID, chunk)
	}
}

// addReaction adds an emoji reaction to a message.
func (b *Bot) addReaction(channelID, timestamp, emoji string) {
	if timestamp == "" {
		return
	}
	if err := b.slackClient.AddReaction(emoji, slack.ItemRef{
		Channel:   channelID,
		Timestamp: timestamp,
	}); err != nil {
		b.logger.Debug("Failed to add reaction", slog.String("error", err.Error()))
	}
}

// removeReaction removes an emoji reaction from a message.
func (b *Bot) removeReaction(channelID, timestamp, emoji string) {
	if timestamp == "" {
		return
	}
	if err := b.slackClient.RemoveReaction(emoji, slack.ItemRef{
		Channel:   channelID,
		Timestamp: timestamp,
	}); err != nil {
		b.logger.Debug("Failed to remove reaction", slog.String("error", err.Error()))
	}
}

// postThinking posts a "Thinking..." message and returns its timestamp.
func (b *Bot) postThinking(cs *chatState) string {
	opts := []slack.MsgOption{slack.MsgOptionText("_Thinking..._", false)}
	if cs.threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(cs.threadTS))
	}

	_, ts, err := b.slackClient.PostMessage(cs.channelID, opts...)
	if err != nil {
		b.logger.Debug("Failed to post thinking message", slog.String("error", err.Error()))
		return ""
	}
	return ts
}

// clearPendingIndicators removes the "thinking" reaction and deletes the
// "Thinking..." message when the first response arrives.
func (b *Bot) clearPendingIndicators(cs *chatState) {
	cs.mu.Lock()
	reaction := cs.pendingReaction
	cs.pendingReaction = nil
	thinking := cs.thinkingMessage
	cs.thinkingMessage = nil
	cs.mu.Unlock()

	if reaction != nil {
		b.removeReaction(reaction.channel, reaction.timestamp, "hourglass_flowing_sand")
	}
	if thinking != nil {
		if _, _, err := b.slackClient.DeleteMessage(thinking.channel, thinking.timestamp); err != nil {
			b.logger.Debug("Failed to delete thinking message", slog.String("error", err.Error()))
		}
	}
}

// ---------------------------------------------------------------------------
// State management
// ---------------------------------------------------------------------------

// isChannelAllowed checks if a channel ID is authorized to use the bot.
func (b *Bot) isChannelAllowed(channelID string) bool {
	_, ok := b.allowedChannels[channelID]
	return ok
}

// getOrCreateChat returns or creates a chatState for the given conversation.
// convKey is the map key (channelID for DMs, "channelID:threadTS" for threads).
func (b *Bot) getOrCreateChat(convKey, channelID, threadTS string) *chatState {
	val, _ := b.chats.LoadOrStore(convKey, &chatState{
		channelID: channelID,
		threadTS:  threadTS,
	})
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
	cs.pendingReaction = nil
	cs.thinkingMessage = nil
}

// userIdentity creates a UserIdentity scoped to a specific conversation.
// Using convKey ensures each conversation (DM, thread) gets its own agent session.
func (b *Bot) userIdentity(convKey, userID string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("slack:%s:%s", convKey, userID),
		Username: "slack",
		Role:     auth.RoleAdmin,
	}
}

// userIdentityFromChannelID creates a minimal UserIdentity from a channel ID.
func (b *Bot) userIdentityFromChannelID(channelID string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("slack:%s", channelID),
		Username: "slack",
		Role:     auth.RoleAdmin,
	}
}

// ---------------------------------------------------------------------------
// Session rotation
// ---------------------------------------------------------------------------

// rotateSession creates a new session carrying forward recent context.
func (b *Bot) rotateSession(ctx context.Context, cs *chatState, user agent.UserIdentity, text string) {
	cs.mu.Lock()
	oldSID := cs.sessionID
	cs.mu.Unlock()

	var summary string
	if oldSID != "" {
		summary = b.buildSessionSummary(ctx, oldSID, user.UserID)
	}

	b.resetChat(cs)
	b.sendReply(cs, "(Session context limit reached — continuing with recent context carried forward)")

	var message string
	if summary != "" {
		message = fmt.Sprintf("[Previous conversation summary:\n%s]\n\n%s", summary, text)
	} else {
		message = text
	}

	b.createSession(ctx, cs, user, message)
}

// buildSessionSummary extracts the last few assistant messages from a session.
func (b *Bot) buildSessionSummary(ctx context.Context, sessionID, userID string) string {
	detail, err := b.agentAPI.GetSessionDetail(ctx, sessionID, userID)
	if err != nil || detail == nil {
		return ""
	}

	const maxExchanges = 3
	var exchanges []string
	var count int

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

	var totalTokens int
	for _, msg := range detail.Messages {
		if msg.Usage != nil {
			totalTokens += msg.Usage.TotalTokens
		}
	}

	limit := int(float64(defaultContextLimit) * contextRotationRatio)
	return totalTokens > limit
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// splitMessage splits text into chunks that fit within the Slack message limit.
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

// stripBotMention removes the bot mention prefix from a message text.
// Slack formats mentions as <@U12345> at the beginning of the text.
func stripBotMention(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<@") {
		if idx := strings.Index(text, ">"); idx >= 0 {
			text = strings.TrimSpace(text[idx+1:])
		}
	}
	return text
}
