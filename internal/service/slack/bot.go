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

// AgentService is the subset of the agent API that the Slack bot requires.
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

type slackClientAPI interface {
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	DeleteMessage(channel, timestamp string) (string, string, error)
	SendMessage(channelID string, options ...slack.MsgOption) (string, string, string, error)
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
	channelID        string // Slack channel ID
	threadTS         string // thread parent timestamp (empty for DMs)
	sessionID        string
	ownerUserID      string // conversation-scoped session identity
	subSessionID     string // session ID the subscription is listening to
	subCancel        context.CancelFunc
	mu               sync.Mutex
	pendingPromptID  string
	thinkingMessage  *messageRef // "Thinking..." message to delete on first response
	lastDeliveredSeq int64
}

// Bot is a Slack bot that forwards messages to the Dagu agent API.
type Bot struct {
	cfg             Config
	agentAPI        AgentService
	slackClient     slackClientAPI
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
		slog.Bool("respond_to_all", b.cfg.RespondToAll),
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
	b.logger.Debug("Socket mode event", slog.String("event_type", string(evt.Type)))

	switch evt.Type { //nolint:exhaustive // only handling relevant event types
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

	default:
		// Ignore other socket mode events (connecting, hello, errors, etc.)
	}
}

// handleEventsAPI processes Events API events (messages, app mentions).
func (b *Bot) handleEventsAPI(ctx context.Context, evt slackevents.EventsAPIEvent) {
	b.logger.Debug("Received event", slog.String("type", evt.InnerEvent.Type))

	switch slackevents.EventsAPIType(evt.InnerEvent.Type) { //nolint:exhaustive // only handling relevant event types
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

		b.logger.Debug("Message event received",
			slog.String("channel", ev.Channel),
			slog.String("channel_type", ev.ChannelType),
			slog.String("user", ev.User),
			slog.String("sub_type", ev.SubType),
			slog.String("bot_id", ev.BotID),
		)

		// Only handle regular messages (not bot messages, edits, etc.)
		if ev.SubType != "" || ev.BotID != "" {
			b.logger.Debug("Skipping message", slog.String("reason", "subtype or bot"))
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
		if b.cfg.RespondToAll && b.isChannelAllowed(ev.Channel) {
			b.logger.Debug("Processing channel message (respond_to_all)")
			b.handleDMMessage(ctx, ev.Channel, ev.User, ev.TimeStamp, ev.Text)
		} else {
			b.logger.Debug("Ignoring channel message",
				slog.Bool("respond_to_all", b.cfg.RespondToAll),
				slog.Bool("channel_allowed", b.isChannelAllowed(ev.Channel)),
			)
		}

	default:
		// Ignore other Events API event types
	}
}

// handleChannelMessage processes a message in a channel (from @mention or thread reply).
func (b *Bot) handleChannelMessage(ctx context.Context, channelID, userID, _, threadTS, text string) {
	if !b.isChannelAllowed(channelID) {
		return
	}

	// Track this thread so future replies don't need @mention
	threadKey := channelID + ":" + threadTS
	b.activeThreads.Store(threadKey, true)

	cs := b.getOrCreateChat(threadKey, channelID, threadTS)
	b.processIncoming(ctx, cs, threadKey, userID, text)
}

// handleDMMessage processes a direct message.
func (b *Bot) handleDMMessage(ctx context.Context, channelID, userID, _, text string) {
	cs := b.getOrCreateChat(channelID, channelID, "")
	b.processIncoming(ctx, cs, channelID, userID, text)
}

// processIncoming is the core message handler shared by channel and DM flows.
// convKey uniquely identifies the conversation (channelID or channelID:threadTS).
func (b *Bot) processIncoming(ctx context.Context, cs *chatState, convKey, _ string, text string) {
	if text == "" {
		return
	}

	// Handle text commands
	if fields := strings.Fields(text); len(fields) > 0 {
		if cmd := fields[0]; cmd == "new" || cmd == "cancel" {
			b.handleTextCommand(ctx, cs, cmd)
			return
		}
	}

	// Check if this is a response to a pending prompt
	cs.mu.Lock()
	pendingPrompt := cs.pendingPromptID
	cs.mu.Unlock()

	if pendingPrompt != "" {
		b.submitPromptResponse(ctx, cs, pendingPrompt, text)
		return
	}

	// Post a typing indicator message
	thinkingTS := b.postThinking(cs)

	user := b.userIdentity(convKey)

	cs.mu.Lock()
	if thinkingTS != "" {
		cs.thinkingMessage = &messageRef{channel: cs.channelID, timestamp: thinkingTS}
	}
	cs.mu.Unlock()

	if cs.sessionID == "" {
		b.createSession(ctx, cs, user, text)
	} else {
		sid, rotated := b.prepareSessionForMessage(ctx, cs, user)
		if sid == "" {
			b.createSession(ctx, cs, user, text)
			return
		}
		if rotated {
			b.clearPendingIndicators(cs)
		}
		b.sendAgentMessage(ctx, cs, user, text)
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

	b.setActiveSession(cs, sessionID, user.UserID)
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
	if maxSeen > lastDelivered {
		b.markDelivered(cs, maxSeen)
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
	if _, err := b.postText(cs.channelID, cs.threadTS, text); err != nil {
		b.logger.Warn("Failed to send Slack message",
			slog.String("channel_id", cs.channelID),
			slog.String("error", err.Error()),
		)
	}
}

// sendText sends a simple text message to a channel (used by monitor for notifications).
func (b *Bot) sendText(channelID, text string) {
	if _, err := b.postText(channelID, "", text); err != nil {
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

// postThinking posts a "Thinking..." message and returns its timestamp.
func (b *Bot) postThinking(cs *chatState) string {
	ts, err := b.postText(cs.channelID, cs.threadTS, "_Thinking..._")
	if err != nil {
		b.logger.Debug("Failed to post thinking message", slog.String("error", err.Error()))
		return ""
	}
	return ts
}

func (b *Bot) postText(channelID, threadTS, text string) (string, error) {
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, ts, err := b.slackClient.PostMessage(channelID, opts...)
	return ts, err
}

func (b *Bot) sendLongTextInThread(channelID, threadTS, text string) {
	chunks := splitMessage(text, maxSlackMessageLen)
	for _, chunk := range chunks {
		if _, err := b.postText(channelID, threadTS, chunk); err != nil {
			b.logger.Warn("Failed to send Slack thread message",
				slog.String("channel_id", channelID),
				slog.String("thread_ts", threadTS),
				slog.String("error", err.Error()),
			)
			return
		}
	}
}

func (b *Bot) sendLongRootThread(channelID, text string) string {
	chunks := splitMessage(text, maxSlackMessageLen)
	if len(chunks) == 0 {
		return ""
	}

	rootTS, err := b.postText(channelID, "", chunks[0])
	if err != nil {
		b.logger.Warn("Failed to send Slack thread root",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()),
		)
		return ""
	}
	for _, chunk := range chunks[1:] {
		if _, err := b.postText(channelID, rootTS, chunk); err != nil {
			b.logger.Warn("Failed to send Slack thread reply",
				slog.String("channel_id", channelID),
				slog.String("thread_ts", rootTS),
				slog.String("error", err.Error()),
			)
			return rootTS
		}
	}
	return rootTS
}

// clearPendingIndicators deletes the "Thinking..." message when the first response arrives.
func (b *Bot) clearPendingIndicators(cs *chatState) {
	cs.mu.Lock()
	thinking := cs.thinkingMessage
	cs.thinkingMessage = nil
	cs.mu.Unlock()

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
	cs.thinkingMessage = nil
	cs.lastDeliveredSeq = 0
}

// userIdentity creates a UserIdentity scoped to a specific conversation.
// Using convKey ensures each conversation (DM, thread) gets its own agent session.
func (b *Bot) userIdentity(convKey string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   fmt.Sprintf("slack:%s", convKey),
		Username: "slack",
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
			b.logger.Warn("Session missing during compaction, resetting conversation",
				slog.String("session", sid),
				slog.String("user", user.UserID),
			)
			b.resetChat(cs)
			return "", false
		}
		b.logger.Warn("Failed to compact Slack session",
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
