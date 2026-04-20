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
	sentComplexMessages  []sentDiscordMessage
	editedMessages       []editedDiscordMessage
	interactionResponses []discordgo.InteractionResponseType
}

type sentDiscordMessage struct {
	content       string
	componentsLen int
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
	componentsLen := 0
	if data != nil {
		content = data.Content
		componentsLen = len(data.Components)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentMessages = append(c.sentMessages, content)
	c.sentComplexMessages = append(c.sentComplexMessages, sentDiscordMessage{
		content:       content,
		componentsLen: componentsLen,
	})
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
	cancelCalls      int
	submitResponses  []agent.UserPromptResponse
	onSubmit         func()
	submitErr        error
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelCalls++
	return nil
}

func (s *fakeDiscordAgentService) SubmitUserResponse(_ context.Context, _ string, _ string, resp agent.UserPromptResponse) error {
	if s.onSubmit != nil {
		s.onSubmit()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.submitErr != nil {
		return s.submitErr
	}
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
	cs.setPendingPrompt(&agent.UserPrompt{PromptID: "prompt-1", AllowFreeText: true})

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
	cs.setPendingPrompt(&agent.UserPrompt{
		PromptID: "prompt-1",
		Options: []agent.UserPromptOption{
			{ID: "option-a", Label: "Option A"},
		},
	})

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
	assert.Equal(t, "Selection received: option-a", client.editedMessages[0].content)
	assert.Equal(t, 0, client.editedMessages[0].componentsLen)
	assert.Empty(t, cs.PendingPromptID())
}

func TestHandleInteractionCreate_RejectsStalePromptClick(t *testing.T) {
	t.Parallel()

	client := &fakeDiscordClient{}
	service := &fakeDiscordAgentService{}
	bot := &Bot{
		cfg:             Config{RespondToAll: false},
		agentAPI:        service,
		client:          client,
		allowedChannels: map[string]struct{}{"C1": {}},
		logger:          testDiscordLogger(),
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	cs.setPendingPrompt(&agent.UserPrompt{
		PromptID: "prompt-2",
		Options: []agent.UserPromptOption{
			{ID: "option-a", Label: "Option A"},
		},
	})

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

	require.Equal(t, []discordgo.InteractionResponseType{discordgo.InteractionResponseDeferredMessageUpdate}, client.interactionResponses)
	assert.Empty(t, client.editedMessages)
	assert.Equal(t, []string{"That prompt is no longer active."}, client.sentMessages)
	service.mu.Lock()
	assert.Empty(t, service.submitResponses)
	service.mu.Unlock()
	assert.Equal(t, "prompt-2", cs.PendingPromptID())
}

func TestSubmitPromptResponse_PreservesPendingPromptOnError(t *testing.T) {
	t.Parallel()

	client := &fakeDiscordClient{}
	service := &fakeDiscordAgentService{submitErr: assert.AnError}
	bot := &Bot{
		agentAPI: service,
		client:   client,
		logger:   testDiscordLogger(),
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	cs.setPendingPrompt(&agent.UserPrompt{PromptID: "prompt-1", AllowFreeText: true})

	bot.submitPromptResponse(context.Background(), cs, "C1", "prompt-1", "free-form answer")

	assert.Equal(t, "prompt-1", cs.PendingPromptID())
	assert.Equal(t, []string{"Failed to submit response: assert.AnError general error for testing"}, client.sentMessages)
}

func TestHandleTextCommand_CancelResetsChatState(t *testing.T) {
	t.Parallel()

	service := &fakeDiscordAgentService{}
	bot := &Bot{
		agentAPI: service,
		client:   &fakeDiscordClient{},
		logger:   testDiscordLogger(),
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	cs.setPendingPrompt(&agent.UserPrompt{PromptID: "prompt-1", AllowFreeText: true})

	bot.handleTextCommand(context.Background(), cs, "C1", "cancel")

	assert.Empty(t, cs.SessionID())
	assert.Empty(t, cs.PendingPromptID())
	service.mu.Lock()
	assert.Equal(t, 1, service.cancelCalls)
	service.mu.Unlock()
}

func TestSendPrompt_OverflowFallsBackToTextOptionIDs(t *testing.T) {
	t.Parallel()

	client := &fakeDiscordClient{}
	service := &fakeDiscordAgentService{}
	bot := &Bot{
		cfg:      Config{RespondToAll: true},
		agentAPI: service,
		client:   client,
		logger:   testDiscordLogger(),
	}

	options := make([]agent.UserPromptOption, 0, maxDiscordPromptOptions+1)
	for i := 1; i <= maxDiscordPromptOptions+1; i++ {
		options = append(options, agent.UserPromptOption{
			ID:    fmt.Sprintf("opt-%02d", i),
			Label: fmt.Sprintf("Option %02d", i),
		})
	}

	cs := bot.getOrCreateChat("C1")
	bot.setActiveSession(cs, "session-1", "discord:C1")
	bot.sendPrompt(cs, "C1", &agent.UserPrompt{
		PromptID: "prompt-1",
		Question: "Choose an option",
		Options:  options,
	})
	bot.submitPromptResponse(context.Background(), cs, "C1", "prompt-1", "opt-26")

	assert.Empty(t, client.sentComplexMessages)
	require.NotEmpty(t, client.sentMessages)
	assert.Contains(t, client.sentMessages[0], "Reply with one of these option IDs:")
	assert.Contains(t, client.sentMessages[0], "opt-26")
	service.mu.Lock()
	defer service.mu.Unlock()
	require.Len(t, service.submitResponses, 1)
	assert.Equal(t, []string{"opt-26"}, service.submitResponses[0].SelectedOptionIDs)
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
				DAGRun: &exec.DAGRunStatus{
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
