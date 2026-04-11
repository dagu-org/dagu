// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package discord

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/chatbridge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDiscordClient struct {
	mu                   sync.Mutex
	sentMessages         []string
	editedMessages       []editedDiscordMessage
	interactionResponses []discordgo.InteractionResponseType
}

type editedDiscordMessage struct {
	channelID     string
	messageID     string
	content       string
	componentsLen int
}

func (c *fakeDiscordClient) ChannelMessageSend(_ string, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentMessages = append(c.sentMessages, content)
	return &discordgo.Message{Content: content}, nil
}

func (c *fakeDiscordClient) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	content := ""
	if data != nil {
		content = data.Content
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentMessages = append(c.sentMessages, content)
	return &discordgo.Message{ID: "sent", ChannelID: channelID, Content: content}, nil
}

func (c *fakeDiscordClient) ChannelMessageEditComplex(m *discordgo.MessageEdit, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	content := ""
	if m.Content != nil {
		content = *m.Content
	}
	componentsLen := -1
	if m.Components != nil {
		componentsLen = len(*m.Components)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.editedMessages = append(c.editedMessages, editedDiscordMessage{
		channelID:     m.Channel,
		messageID:     m.ID,
		content:       content,
		componentsLen: componentsLen,
	})
	return &discordgo.Message{ID: m.ID, ChannelID: m.Channel, Content: content}, nil
}

func (c *fakeDiscordClient) ChannelTyping(string, ...discordgo.RequestOption) error {
	return nil
}

func (c *fakeDiscordClient) InteractionRespond(_ *discordgo.Interaction, resp *discordgo.InteractionResponse, _ ...discordgo.RequestOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interactionResponses = append(c.interactionResponses, resp.Type)
	return nil
}

func (c *fakeDiscordClient) interactionResponseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.interactionResponses)
}

type fakeDiscordAgentService struct {
	mu               sync.Mutex
	nextSessionID    int
	nextSequenceID   int64
	createEmptyCalls int
	appendSessionIDs []string
	submitResponses  []agent.UserPromptResponse
	onSubmit         func()
}

func (s *fakeDiscordAgentService) CreateSession(_ context.Context, _ agent.UserIdentity, _ agent.ChatRequest) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	return fmt.Sprintf("created-%d", s.nextSessionID), "", nil
}

func (s *fakeDiscordAgentService) CreateEmptySession(_ context.Context, _ agent.UserIdentity, _ string, _ bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	s.createEmptyCalls++
	return fmt.Sprintf("notify-%d", s.nextSessionID), nil
}

func (s *fakeDiscordAgentService) SendMessage(context.Context, string, agent.UserIdentity, agent.ChatRequest) error {
	return nil
}

func (s *fakeDiscordAgentService) EnqueueChatMessage(_ context.Context, sessionID string, _ agent.UserIdentity, _ agent.ChatRequest) (agent.ChatQueueResult, error) {
	return agent.ChatQueueResult{SessionID: sessionID, Started: true}, nil
}

func (s *fakeDiscordAgentService) FlushQueuedChatMessage(_ context.Context, sessionID string, _ agent.UserIdentity) (agent.ChatQueueResult, error) {
	return agent.ChatQueueResult{SessionID: sessionID, Started: true}, nil
}

func (s *fakeDiscordAgentService) CancelSession(context.Context, string, string) error {
	return nil
}

func (s *fakeDiscordAgentService) SubmitUserResponse(_ context.Context, _ string, _ string, resp agent.UserPromptResponse) error {
	if s.onSubmit != nil {
		s.onSubmit()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submitResponses = append(s.submitResponses, resp)
	return nil
}

func (s *fakeDiscordAgentService) GenerateAssistantMessage(context.Context, string, agent.UserIdentity, string, string) (agent.Message, error) {
	return agent.Message{
		Type:      agent.MessageTypeAssistant,
		Content:   "notification",
		CreatedAt: time.Now(),
	}, nil
}

func (s *fakeDiscordAgentService) AppendExternalMessage(_ context.Context, sessionID string, _ agent.UserIdentity, msg agent.Message) (agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg.SessionID = sessionID
	msg.SequenceID = s.nextSequenceID
	s.nextSequenceID++
	s.appendSessionIDs = append(s.appendSessionIDs, sessionID)
	return msg, nil
}

func (s *fakeDiscordAgentService) CompactSessionIfNeeded(_ context.Context, sessionID string, _ agent.UserIdentity) (string, bool, error) {
	return sessionID, false, nil
}

func (s *fakeDiscordAgentService) GetSessionDetail(context.Context, string, string) (*agent.StreamResponse, error) {
	return &agent.StreamResponse{}, nil
}

func (s *fakeDiscordAgentService) SubscribeSession(context.Context, string, agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error) {
	return agent.StreamResponse{}, func() (agent.StreamResponse, bool) { return agent.StreamResponse{}, false }, nil
}

func testDiscordLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandleMessageCreate_StillRequiresMentionWithoutPendingPrompt(t *testing.T) {
	t.Parallel()

	bot := &Bot{
		cfg:             Config{RespondToAll: false},
		agentAPI:        &fakeDiscordAgentService{},
		client:          &fakeDiscordClient{},
		selfID:          "bot-1",
		allowedChannels: map[string]struct{}{"C1": {}},
		logger:          testDiscordLogger(),
	}

	bot.handleMessageCreate(context.Background(), &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Author:    &discordgo.User{ID: "user-1"},
			ChannelID: "C1",
			GuildID:   "G1",
			Content:   "hello",
		},
	})

	_, exists := bot.chats.Load("C1")
	assert.False(t, exists)
}

func TestHandleMessageCreate_AllowsPendingPromptReplyWithoutMention(t *testing.T) {
	t.Parallel()

	service := &fakeDiscordAgentService{}
	bot := &Bot{
		cfg:             Config{RespondToAll: false},
		agentAPI:        service,
		client:          &fakeDiscordClient{},
		selfID:          "bot-1",
		allowedChannels: map[string]struct{}{"C1": {}},
		logger:          testDiscordLogger(),
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	cs.SetPendingPrompt("prompt-1")

	bot.handleMessageCreate(context.Background(), &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Author:    &discordgo.User{ID: "user-1"},
			ChannelID: "C1",
			GuildID:   "G1",
			Content:   "free-form answer",
		},
	})

	service.mu.Lock()
	defer service.mu.Unlock()
	require.Len(t, service.submitResponses, 1)
	assert.Equal(t, "prompt-1", service.submitResponses[0].PromptID)
	assert.Equal(t, "free-form answer", service.submitResponses[0].FreeTextResponse)
}

func TestHandleInteractionCreate_AcksBeforeSubmittingAndEditsMessage(t *testing.T) {
	t.Parallel()

	client := &fakeDiscordClient{}
	service := &fakeDiscordAgentService{}
	seenResponses := 0
	service.onSubmit = func() {
		seenResponses = client.interactionResponseCount()
	}

	bot := &Bot{
		cfg:             Config{RespondToAll: false},
		agentAPI:        service,
		client:          client,
		allowedChannels: map[string]struct{}{"C1": {}},
		logger:          testDiscordLogger(),
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	cs.SetPendingPrompt("prompt-1")

	bot.handleInteractionCreate(context.Background(), &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-1",
			Type:      discordgo.InteractionMessageComponent,
			ChannelID: "C1",
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "prompt:prompt-1:option-a",
			},
			Message: &discordgo.Message{
				ID:      "message-1",
				Content: "Choose an option",
			},
		},
	})

	assert.Equal(t, 1, seenResponses, "interaction should be acknowledged before the agent call")
	require.Equal(t, []discordgo.InteractionResponseType{discordgo.InteractionResponseDeferredMessageUpdate}, client.interactionResponses)
	require.Len(t, client.editedMessages, 1)
	assert.Equal(t, "C1", client.editedMessages[0].channelID)
	assert.Equal(t, "message-1", client.editedMessages[0].messageID)
	assert.Contains(t, client.editedMessages[0].content, "Selected: option-a")
	assert.Equal(t, 0, client.editedMessages[0].componentsLen)
}

func TestDAGRunMonitor_UsesDedicatedNotificationState(t *testing.T) {
	t.Parallel()

	client := &fakeDiscordClient{}
	service := &fakeDiscordAgentService{nextSequenceID: 1}
	bot := &Bot{
		cfg:             Config{SafeMode: true},
		agentAPI:        service,
		client:          client,
		allowedChannels: map[string]struct{}{"C1": {}},
		logger:          testDiscordLogger(),
	}

	liveChat := bot.getOrCreateChat("C1")
	bot.setActiveSession(liveChat, "live-session", "discord:C1")

	monitor := &DAGRunMonitor{
		agentAPI: service,
		bot:      bot,
		logger:   testDiscordLogger(),
	}

	ok := monitor.flushChannel(context.Background(), "C1", chatbridge.NotificationBatch{
		Class: chatbridge.NotificationClassSuccessDigest,
		Events: []chatbridge.NotificationEvent{
			{
				Status: &exec.DAGRunStatus{
					Name:   "build",
					Status: core.Succeeded,
				},
			},
		},
	}, false)
	require.True(t, ok)

	service.mu.Lock()
	require.Len(t, service.appendSessionIDs, 1)
	assert.Equal(t, 1, service.createEmptyCalls)
	assert.NotEqual(t, "live-session", service.appendSessionIDs[0])
	notificationSessionID := service.appendSessionIDs[0]
	service.mu.Unlock()

	assert.Equal(t, "live-session", liveChat.SessionID())
	assert.Equal(t, notificationSessionID, bot.getOrCreateNotificationChat("C1").SessionID())
	assert.Len(t, client.sentMessages, 1)
}
