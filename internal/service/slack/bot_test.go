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
	mu      sync.Mutex
	postTS  int
	posts   []string
	deletes int
}

func (c *fakeSlackClient) PostMessage(_ string, _ ...slack.MsgOption) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.postTS++
	ts := fmt.Sprintf("%d", c.postTS)
	c.posts = append(c.posts, ts)
	return "ok", ts, nil
}

func (c *fakeSlackClient) DeleteMessage(_, _ string) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletes++
	return "ok", "", nil
}

func (c *fakeSlackClient) SendMessage(_ string, _ ...slack.MsgOption) (string, string, string, error) {
	return "ok", "", "", nil
}

func (c *fakeSlackClient) postCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.posts)
}

type fakeSlackAgentService struct {
	mu               sync.Mutex
	nextSessionID    int
	nextSequenceID   int64
	createEmptyCalls int
	appendSessionIDs []string
	generated        agent.Message
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

func (s *fakeSlackAgentService) CreateSession(context.Context, agent.UserIdentity, agent.ChatRequest) (string, string, error) {
	return "created", "", nil
}

func (s *fakeSlackAgentService) CreateEmptySession(_ context.Context, _ agent.UserIdentity, _ string, _ bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	s.createEmptyCalls++
	return fmt.Sprintf("sess-%d", s.nextSessionID), nil
}

func (s *fakeSlackAgentService) SendMessage(context.Context, string, agent.UserIdentity, agent.ChatRequest) error {
	return nil
}

func (s *fakeSlackAgentService) CancelSession(context.Context, string, string) error {
	return nil
}

func (s *fakeSlackAgentService) SubmitUserResponse(context.Context, string, string, agent.UserPromptResponse) error {
	return nil
}

func (s *fakeSlackAgentService) GenerateAssistantMessage(context.Context, string, agent.UserIdentity, string, string) (agent.Message, error) {
	return s.generated, nil
}

func (s *fakeSlackAgentService) AppendExternalMessage(_ context.Context, sessionID string, _ agent.UserIdentity, msg agent.Message) (agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg.SessionID = sessionID
	msg.SequenceID = s.nextSequenceID
	s.nextSequenceID++
	s.appendSessionIDs = append(s.appendSessionIDs, sessionID)
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

func TestDAGRunMonitor_NotifyChannelThread_SkipsManualNotificationReplay(t *testing.T) {
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
	monitor := NewDAGRunMonitor(nil, service, bot, logger)

	ok := monitor.notifyChannel(context.Background(), "C123", &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
	}, "prompt")
	require.True(t, ok)
	assert.Equal(t, 1, client.postCount(), "notification should be delivered once as a thread root")

	val, exists := bot.chats.Load("C123:1")
	require.True(t, exists)
	cs := val.(*chatState)
	assert.Equal(t, "sess-1", cs.sessionID)
	assert.Equal(t, "1", cs.threadTS)
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))

	bot.processStreamResponse(cs, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "thread notification"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "follow-up answer"},
		},
	})

	assert.Equal(t, 2, client.postCount(), "snapshot replay should skip the already delivered notification")
}

func TestDAGRunMonitor_NotifyDirectMessage_AppendsToExistingSession(t *testing.T) {
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
	monitor := NewDAGRunMonitor(nil, service, bot, logger)

	cs := bot.getOrCreateChat("D123", "D123", "")
	user := agent.UserIdentity{
		UserID:   "slack:D123",
		Username: "slack",
		Role:     auth.RoleAdmin,
	}
	bot.setActiveSession(cs, "existing-session", user.UserID)

	ok := monitor.notifyChannel(context.Background(), "D123", &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
	}, "prompt")
	require.True(t, ok)

	assert.Equal(t, 0, service.createEmptyCalls, "existing DM session should be reused")
	require.Len(t, service.appendSessionIDs, 1)
	assert.Equal(t, "existing-session", service.appendSessionIDs[0])
	assert.Equal(t, "existing-session", cs.sessionID)
	assert.Equal(t, int64(1), bot.lastDeliveredSeq(cs))
	assert.Equal(t, 1, client.postCount(), "notification should still be delivered to Slack once")

	bot.processStreamResponse(cs, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "dm notification"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "actual reply"},
		},
	})

	assert.Equal(t, 2, client.postCount(), "replayed notification should be suppressed on the next snapshot")
}
