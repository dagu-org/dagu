// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTelegramAPI struct {
	mu    sync.Mutex
	sends []tgbotapi.Chattable
}

func (a *fakeTelegramAPI) GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (a *fakeTelegramAPI) StopReceivingUpdates() {}

func (a *fakeTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sends = append(a.sends, c)
	return tgbotapi.Message{}, nil
}

func (a *fakeTelegramAPI) sendCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sends)
}

type fakeTelegramAgentService struct {
	mu               sync.Mutex
	nextSessionID    int
	nextSequenceID   int64
	createEmptyCalls int
	appendSessionIDs []string
	createMessages   []string
	sendMessages     []string
	generated        agent.Message
}

func newFakeTelegramAgentService(content string) *fakeTelegramAgentService {
	return &fakeTelegramAgentService{
		nextSequenceID: 1,
		generated: agent.Message{
			Type:      agent.MessageTypeAssistant,
			Content:   content,
			CreatedAt: time.Now(),
			LLMData: &llm.Message{
				Role:    llm.RoleAssistant,
				Content: content,
			},
		},
	}
}

func (s *fakeTelegramAgentService) CreateSession(_ context.Context, _ agent.UserIdentity, req agent.ChatRequest) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createMessages = append(s.createMessages, req.Message)
	s.nextSessionID++
	return fmt.Sprintf("created-%d", s.nextSessionID), "", nil
}

func (s *fakeTelegramAgentService) CreateEmptySession(_ context.Context, _ agent.UserIdentity, _ string, _ bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	s.createEmptyCalls++
	return fmt.Sprintf("sess-%d", s.nextSessionID), nil
}

func (s *fakeTelegramAgentService) SendMessage(_ context.Context, _ string, _ agent.UserIdentity, req agent.ChatRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendMessages = append(s.sendMessages, req.Message)
	return nil
}

func (s *fakeTelegramAgentService) CancelSession(context.Context, string, string) error {
	return nil
}

func (s *fakeTelegramAgentService) SubmitUserResponse(context.Context, string, string, agent.UserPromptResponse) error {
	return nil
}

func (s *fakeTelegramAgentService) GenerateAssistantMessage(context.Context, string, agent.UserIdentity, string, string) (agent.Message, error) {
	return s.generated, nil
}

func (s *fakeTelegramAgentService) AppendExternalMessage(_ context.Context, sessionID string, _ agent.UserIdentity, msg agent.Message) (agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg.SessionID = sessionID
	msg.SequenceID = s.nextSequenceID
	s.nextSequenceID++
	s.appendSessionIDs = append(s.appendSessionIDs, sessionID)
	return msg, nil
}

func (s *fakeTelegramAgentService) CompactSessionIfNeeded(_ context.Context, sessionID string, _ agent.UserIdentity) (string, bool, error) {
	return sessionID, false, nil
}

func (s *fakeTelegramAgentService) GetSessionDetail(context.Context, string, string) (*agent.StreamResponse, error) {
	return &agent.StreamResponse{}, nil
}

func (s *fakeTelegramAgentService) SubscribeSession(context.Context, string, agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error) {
	return agent.StreamResponse{}, func() (agent.StreamResponse, bool) { return agent.StreamResponse{}, false }, nil
}

func TestDAGRunMonitor_NotifyChat_ReusesExistingSessionAndSkipsReplay(t *testing.T) {
	t.Parallel()

	api := &fakeTelegramAPI{}
	service := newFakeTelegramAgentService("telegram notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:          Config{SafeMode: true},
		agentAPI:     service,
		botAPI:       api,
		allowedChats: map[int64]struct{}{123: {}},
		logger:       logger,
	}
	monitor := NewDAGRunMonitor(nil, service, bot, logger)

	cs := bot.getOrCreateChat(123)
	bot.setActiveSession(cs, "existing-session", "telegram:123")

	ok := monitor.notifyChat(context.Background(), 123, &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
	}, "prompt")
	require.True(t, ok)

	assert.Equal(t, 0, service.createEmptyCalls, "existing chat session should be reused")
	require.Len(t, service.appendSessionIDs, 1)
	assert.Equal(t, "existing-session", service.appendSessionIDs[0])
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))
	assert.Equal(t, 1, api.sendCount())

	bot.processStreamResponse(cs, 123, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "telegram notification"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "actual reply"},
		},
	})

	assert.Equal(t, 2, api.sendCount(), "the manually delivered notification must not be replayed")
}

func TestDAGRunMonitor_NotifyChat_CreatesSessionWhenMissing(t *testing.T) {
	t.Parallel()

	api := &fakeTelegramAPI{}
	service := newFakeTelegramAgentService("fresh notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:          Config{SafeMode: true},
		agentAPI:     service,
		botAPI:       api,
		allowedChats: map[int64]struct{}{456: {}},
		logger:       logger,
	}
	monitor := NewDAGRunMonitor(nil, service, bot, logger)

	ok := monitor.notifyChat(context.Background(), 456, &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
	}, "prompt")
	require.True(t, ok)

	cs := bot.getOrCreateChat(456)
	assert.Equal(t, "sess-1", cs.sessionID)
	assert.Equal(t, "telegram:456", cs.ownerUserID)
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))
	assert.Equal(t, 1, service.createEmptyCalls)
	assert.Equal(t, 1, api.sendCount())
}

func TestBot_HandleMessage_BatchesRapidMessagesIntoSingleCreate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := &fakeTelegramAPI{}
	service := newFakeTelegramAgentService("ignored")
	bot := &Bot{
		cfg:           Config{SafeMode: true},
		agentAPI:      service,
		botAPI:        api,
		allowedChats:  map[int64]struct{}{123: {}},
		logger:        logger,
		incomingDelay: 10 * time.Millisecond,
	}

	first := &tgbotapi.Message{Text: "first", Chat: &tgbotapi.Chat{ID: 123}}
	second := &tgbotapi.Message{Text: "second", Chat: &tgbotapi.Chat{ID: 123}}
	bot.handleMessage(context.Background(), first)
	bot.handleMessage(context.Background(), second)

	require.Eventually(t, func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		return len(service.createMessages) == 1
	}, time.Second, 10*time.Millisecond)

	service.mu.Lock()
	defer service.mu.Unlock()
	require.Len(t, service.createMessages, 1)
	assert.Equal(t, "first\n\nsecond", service.createMessages[0])
}
