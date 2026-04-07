// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAgentService struct {
	nextSessionID int

	appendErrBySession map[string]error
	appendCalls        []string
	generatedMessage   agent.Message
	generateErr        error

	compactSessionID string
	compactRotated   bool
	compactErr       error

	detail *agent.StreamResponse
}

func (f *fakeAgentService) CreateSession(context.Context, agent.UserIdentity, agent.ChatRequest) (string, string, error) {
	return "", "", nil
}

func (f *fakeAgentService) CreateEmptySession(context.Context, agent.UserIdentity, string, bool) (string, error) {
	f.nextSessionID++
	return fmt.Sprintf("sess-%d", f.nextSessionID), nil
}

func (f *fakeAgentService) SendMessage(context.Context, string, agent.UserIdentity, agent.ChatRequest) error {
	return nil
}

func (f *fakeAgentService) EnqueueChatMessage(context.Context, string, agent.UserIdentity, agent.ChatRequest) (agent.ChatQueueResult, error) {
	return agent.ChatQueueResult{}, nil
}

func (f *fakeAgentService) FlushQueuedChatMessage(context.Context, string, agent.UserIdentity) (agent.ChatQueueResult, error) {
	return agent.ChatQueueResult{}, nil
}

func (f *fakeAgentService) CancelSession(context.Context, string, string) error {
	return nil
}

func (f *fakeAgentService) SubmitUserResponse(context.Context, string, string, agent.UserPromptResponse) error {
	return nil
}

func (f *fakeAgentService) GenerateAssistantMessage(context.Context, string, agent.UserIdentity, string, string) (agent.Message, error) {
	if f.generateErr != nil {
		return agent.Message{}, f.generateErr
	}
	return f.generatedMessage, nil
}

func (f *fakeAgentService) AppendExternalMessage(_ context.Context, sessionID string, _ agent.UserIdentity, msg agent.Message) (agent.Message, error) {
	if err := f.appendErrBySession[sessionID]; err != nil {
		return agent.Message{}, err
	}
	f.appendCalls = append(f.appendCalls, sessionID)
	msg.SessionID = sessionID
	msg.SequenceID = int64(len(f.appendCalls))
	return msg, nil
}

func (f *fakeAgentService) CompactSessionIfNeeded(context.Context, string, agent.UserIdentity) (string, bool, error) {
	return f.compactSessionID, f.compactRotated, f.compactErr
}

func (f *fakeAgentService) GetSessionDetail(context.Context, string, string) (*agent.StreamResponse, error) {
	return f.detail, nil
}

func (f *fakeAgentService) SubscribeSession(context.Context, string, agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error) {
	return agent.StreamResponse{}, func() (agent.StreamResponse, bool) { return agent.StreamResponse{}, false }, nil
}

func TestState_TakePendingMessagesUsesLatestGeneration(t *testing.T) {
	t.Parallel()

	var state State
	oldGen := state.EnqueuePendingMessage("first")
	newGen := state.EnqueuePendingMessage("second")

	text, ok := state.TakePendingMessages(oldGen, "\n\n")
	require.False(t, ok)
	assert.Empty(t, text)

	text, ok = state.TakePendingMessages(newGen, "\n\n")
	require.True(t, ok)
	assert.Equal(t, "first\n\nsecond", text)
}

func TestProcessStreamResponseSkipsDeliveredMessages(t *testing.T) {
	t.Parallel()

	var state State
	state.MarkDelivered(1)

	var assistants []string
	var prompts []string
	ProcessStreamResponse(&state, agent.StreamResponse{
		Messages: []agent.Message{
			{Type: agent.MessageTypeAssistant, SequenceID: 1, Content: "old"},
			{Type: agent.MessageTypeAssistant, SequenceID: 2, Content: "new"},
			{Type: agent.MessageTypeUserPrompt, SequenceID: 3, UserPrompt: &agent.UserPrompt{PromptID: "p1", Question: "Q?"}},
		},
	}, StreamHandlers{
		OnAssistant: func(msg agent.Message) {
			assistants = append(assistants, msg.Content)
		},
		OnPrompt: func(prompt *agent.UserPrompt) {
			prompts = append(prompts, prompt.PromptID)
		},
	})

	assert.Equal(t, []string{"new"}, assistants)
	assert.Equal(t, []string{"p1"}, prompts)
	assert.Equal(t, int64(3), state.LastDeliveredSeq())
}

func TestProcessStreamResponse_TracksQueuedSessionState(t *testing.T) {
	t.Parallel()

	var state State
	ProcessStreamResponse(&state, agent.StreamResponse{
		SessionState: &agent.SessionState{
			SessionID:          "sess-1",
			Working:            false,
			HasQueuedUserInput: true,
		},
	}, StreamHandlers{})

	assert.True(t, state.HasQueuedUserInput())
}

func TestAppendNotificationRecreatesMissingSession(t *testing.T) {
	t.Parallel()

	service := &fakeAgentService{
		appendErrBySession: map[string]error{
			"stale-session": agent.ErrSessionNotFound,
		},
	}
	var state State
	user := agent.UserIdentity{UserID: "slack:C1"}
	state.SetActiveSession("stale-session", user.UserID)

	sessionID, stored, err := AppendNotification(context.Background(), service, &state, user, "briefing", true, agent.Message{
		Type:    agent.MessageTypeAssistant,
		Content: "notification",
	})
	require.NoError(t, err)
	assert.Equal(t, "sess-1", sessionID)
	assert.Equal(t, "sess-1", stored.SessionID)
	assert.Equal(t, "sess-1", state.SessionID())
	assert.Equal(t, []string{"sess-1"}, service.appendCalls)
}

func TestMaybeCompactSessionMarksContinuationSnapshotDelivered(t *testing.T) {
	t.Parallel()

	service := &fakeAgentService{
		compactSessionID: "sess-2",
		compactRotated:   true,
		detail: &agent.StreamResponse{
			Messages: []agent.Message{
				{SequenceID: 4},
				{SequenceID: 9},
			},
		},
	}
	var state State
	user := agent.UserIdentity{UserID: "telegram:123"}
	state.SetActiveSession("sess-1", user.UserID)

	result, err := MaybeCompactSession(context.Background(), service, &state, user)
	require.NoError(t, err)
	assert.False(t, result.Missing)
	assert.True(t, result.Rotated)
	assert.Equal(t, "sess-2", result.SessionID)
	assert.Equal(t, "sess-2", state.SessionID())
	assert.Equal(t, int64(9), state.LastDeliveredSeq())
}

func TestMaybeCompactSessionReportsMissingSession(t *testing.T) {
	t.Parallel()

	service := &fakeAgentService{
		compactErr: errors.New("boom"),
	}
	var state State
	user := agent.UserIdentity{UserID: "telegram:123"}
	state.SetActiveSession("sess-1", user.UserID)

	result, err := MaybeCompactSession(context.Background(), service, &state, user)
	require.Error(t, err)
	assert.False(t, result.Rotated)
	assert.False(t, result.Missing)
	assert.Equal(t, "sess-1", result.SessionID)

	service.compactErr = agent.ErrSessionNotFound
	result, err = MaybeCompactSession(context.Background(), service, &state, user)
	require.NoError(t, err)
	assert.False(t, result.Rotated)
	assert.True(t, result.Missing)
	assert.Equal(t, "sess-1", result.SessionID)
}
