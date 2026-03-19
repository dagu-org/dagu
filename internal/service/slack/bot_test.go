// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package slack

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSlackClient struct {
	mu           sync.Mutex
	postTS       int
	posts        []string
	postChannels []string
	postAttempts map[string]int
	failChannels map[string]int
	deletes      int
}

func (c *fakeSlackClient) PostMessage(channel string, _ ...slack.MsgOption) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.postAttempts == nil {
		c.postAttempts = make(map[string]int)
	}
	c.postAttempts[channel]++
	if remaining := c.failChannels[channel]; remaining > 0 {
		c.failChannels[channel] = remaining - 1
		return "", "", assert.AnError
	}
	c.postTS++
	ts := fmt.Sprintf("%d", c.postTS)
	c.posts = append(c.posts, ts)
	c.postChannels = append(c.postChannels, channel)
	return "ok", ts, nil
}

func (c *fakeSlackClient) DeleteMessage(_, _ string) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletes++
	return "ok", "", nil
}

//revive:disable-next-line:function-result-limit // matches slackClientAPI
func (c *fakeSlackClient) SendMessage(_ string, _ ...slack.MsgOption) (string, string, string, error) {
	return "ok", "", "", nil
}

func (c *fakeSlackClient) postCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.posts)
}

func (c *fakeSlackClient) deleteCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deletes
}

func (c *fakeSlackClient) attemptsForChannel(channel string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.postAttempts[channel]
}

type fakeSlackAgentService struct {
	mu               sync.Mutex
	nextSessionID    int
	nextSequenceID   int64
	createEmptyCalls int
	appendAttempts   []string
	appendSessionIDs []string
	appendMessages   []agent.Message
	createMessages   []string
	sendMessages     []string
	flushCalls       int
	generateCalls    int
	generated        agent.Message
	generatedErr     error
	enqueueResult    agent.ChatQueueResult
	flushResult      agent.ChatQueueResult
}

func newFakeSlackAgentService(content string) *fakeSlackAgentService {
	return &fakeSlackAgentService{
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

func (s *fakeSlackAgentService) CreateSession(_ context.Context, _ agent.UserIdentity, req agent.ChatRequest) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createMessages = append(s.createMessages, req.Message)
	s.nextSessionID++
	return fmt.Sprintf("created-%d", s.nextSessionID), "", nil
}

func (s *fakeSlackAgentService) CreateEmptySession(_ context.Context, _ agent.UserIdentity, _ string, _ bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	s.createEmptyCalls++
	return fmt.Sprintf("sess-%d", s.nextSessionID), nil
}

func (s *fakeSlackAgentService) SendMessage(_ context.Context, _ string, _ agent.UserIdentity, req agent.ChatRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendMessages = append(s.sendMessages, req.Message)
	return nil
}

func (s *fakeSlackAgentService) EnqueueChatMessage(_ context.Context, sessionID string, _ agent.UserIdentity, req agent.ChatRequest) (agent.ChatQueueResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendMessages = append(s.sendMessages, req.Message)
	result := s.enqueueResult
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	if !result.Queued {
		result.Started = true
	}
	return result, nil
}

func (s *fakeSlackAgentService) FlushQueuedChatMessage(_ context.Context, sessionID string, _ agent.UserIdentity) (agent.ChatQueueResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushCalls++
	result := s.flushResult
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	return result, nil
}

func (s *fakeSlackAgentService) CancelSession(context.Context, string, string) error {
	return nil
}

func (s *fakeSlackAgentService) SubmitUserResponse(context.Context, string, string, agent.UserPromptResponse) error {
	return nil
}

func (s *fakeSlackAgentService) GenerateAssistantMessage(context.Context, string, agent.UserIdentity, string, string) (agent.Message, error) {
	s.mu.Lock()
	s.generateCalls++
	s.mu.Unlock()
	if s.generatedErr != nil {
		return agent.Message{}, s.generatedErr
	}
	return s.generated, nil
}

func (s *fakeSlackAgentService) AppendExternalMessage(_ context.Context, sessionID string, _ agent.UserIdentity, msg agent.Message) (agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendAttempts = append(s.appendAttempts, sessionID)
	msg.SessionID = sessionID
	msg.SequenceID = s.nextSequenceID
	s.nextSequenceID++
	s.appendSessionIDs = append(s.appendSessionIDs, sessionID)
	s.appendMessages = append(s.appendMessages, msg)
	return msg, nil
}

func (s *fakeSlackAgentService) CompactSessionIfNeeded(_ context.Context, sessionID string, _ agent.UserIdentity) (string, bool, error) {
	return sessionID, false, nil
}

func (s *fakeSlackAgentService) GetSessionDetail(context.Context, string, string) (*agent.StreamResponse, error) {
	return &agent.StreamResponse{}, nil
}

func (s *fakeSlackAgentService) SubscribeSession(context.Context, string, agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error) {
	return agent.StreamResponse{}, func() (agent.StreamResponse, bool) { return agent.StreamResponse{}, false }, nil
}

func TestDAGRunMonitor_FlushesSuccessDigestIntoSingleThreadAndSkipsReplay(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{}
	service := newFakeSlackAgentService("thread notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:             Config{SafeMode: true},
		agentAPI:        service,
		slackClient:     client,
		allowedChannels: map[string]struct{}{"C123": {}},
		logger:          logger,
	}
	monitor := newDAGRunMonitorWithWindows(nil, service, bot, logger, 10*time.Millisecond, 20*time.Millisecond)
	stopMonitor := startTestMonitor(t, monitor)
	defer stopMonitor()

	ok := monitor.notifyCompletion(context.Background(), &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
	})
	require.True(t, ok)
	ok = monitor.notifyCompletion(context.Background(), &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
	})
	require.True(t, ok)

	require.Eventually(t, func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		return len(service.appendMessages) == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, client.postCount(), "digest should be delivered once as a single thread root")

	val, exists := bot.chats.Load("C123:1")
	require.True(t, exists)
	cs := val.(*chatState)
	assert.Equal(t, "sess-1", cs.SessionID())
	assert.Equal(t, "1", cs.threadTS)
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))
	service.mu.Lock()
	require.Len(t, service.appendMessages, 1)
	assert.Contains(t, service.appendMessages[0].Content, "DAG completion digest")
	assert.Contains(t, service.appendMessages[0].Content, "briefing: succeeded x2")
	service.mu.Unlock()

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "digest"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "follow-up answer"},
		},
	})

	assert.Equal(t, 2, client.postCount(), "snapshot replay should skip the already delivered digest")
}

func TestDAGRunMonitor_FlushesUrgentSingleIntoExistingDMSession(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{}
	service := newFakeSlackAgentService("dm notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:             Config{SafeMode: true},
		agentAPI:        service,
		slackClient:     client,
		allowedChannels: map[string]struct{}{"D123": {}},
		logger:          logger,
	}
	monitor := newDAGRunMonitorWithWindows(nil, service, bot, logger, 10*time.Millisecond, 20*time.Millisecond)
	stopMonitor := startTestMonitor(t, monitor)
	defer stopMonitor()

	cs := bot.getOrCreateChat("D123", "D123", "")
	user := agent.UserIdentity{
		UserID:   "slack:D123",
		Username: "slack",
		Role:     auth.RoleAdmin,
	}
	bot.setActiveSession(cs, "existing-session", user.UserID)

	ok := monitor.notifyCompletion(context.Background(), &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
		Error:     "boom",
	})
	require.True(t, ok)

	require.Eventually(t, func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		return len(service.appendMessages) == 1
	}, time.Second, 10*time.Millisecond)

	service.mu.Lock()
	assert.Equal(t, 0, service.createEmptyCalls, "existing DM session should be reused")
	require.Len(t, service.appendSessionIDs, 1)
	assert.Equal(t, "existing-session", service.appendSessionIDs[0])
	service.mu.Unlock()
	assert.Equal(t, "existing-session", cs.SessionID())
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))
	assert.Equal(t, 1, client.postCount(), "notification should still be delivered to Slack once")
	service.mu.Lock()
	require.Len(t, service.appendMessages, 1)
	assert.Equal(t, "dm notification", service.appendMessages[0].Content)
	service.mu.Unlock()

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "dm notification"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "actual reply"},
		},
	})

	assert.Equal(t, 2, client.postCount(), "replayed notification should be suppressed on the next snapshot")
}

func TestBot_ProcessIncoming_BatchesRapidMessagesIntoSingleCreate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := &fakeSlackClient{}
	service := newFakeSlackAgentService("ignored")
	bot := &Bot{
		cfg:           Config{SafeMode: true},
		agentAPI:      service,
		slackClient:   client,
		logger:        logger,
		incomingDelay: 10 * time.Millisecond,
	}

	cs := bot.getOrCreateChat("D123", "D123", "")
	bot.processIncoming(context.Background(), cs, "D123", "", "first")
	bot.processIncoming(context.Background(), cs, "D123", "", "second")

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

func TestBot_ProcessStreamResponse_RestartsThinkingWhenWorkContinues(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		slackClient: client,
		logger:      logger,
	}
	cs := bot.getOrCreateChat("D123", "D123", "")

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		SessionState: &agent.SessionState{Working: true},
	})
	assert.Equal(t, 1, client.postCount(), "working state should post a thinking indicator")

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "partial reply"},
		},
	})
	assert.Equal(t, 2, client.postCount(), "assistant message should be delivered")
	assert.Equal(t, 1, client.deleteCount(), "assistant message should clear the current thinking indicator")

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		SessionState: &agent.SessionState{Working: true},
	})
	assert.Equal(t, 3, client.postCount(), "continued work should post a fresh thinking indicator")

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		SessionState: &agent.SessionState{Working: true},
	})
	assert.Equal(t, 3, client.postCount(), "duplicate working pulses should not stack thinking indicators")
}

func TestBot_ProcessStreamResponse_FlushesQueuedTurnWhenIdle(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{}
	service := newFakeSlackAgentService("ignored")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		agentAPI:    service,
		slackClient: client,
		logger:      logger,
	}
	cs := bot.getOrCreateChat("D123", "D123", "")
	bot.setActiveSession(cs, "existing-session", "slack:D123")

	bot.processStreamResponse(context.Background(), cs, agent.StreamResponse{
		SessionState: &agent.SessionState{
			SessionID:          "existing-session",
			Working:            false,
			HasQueuedUserInput: true,
		},
	})

	require.Eventually(t, func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		return service.flushCalls == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, client.postCount(), "queued idle state should keep a visible thinking indicator")
}
