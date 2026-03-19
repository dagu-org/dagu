// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
)

// AgentService is the subset of the agent API required by chat bridge adapters.
type AgentService interface {
	CreateSession(ctx context.Context, user agent.UserIdentity, req agent.ChatRequest) (string, string, error)
	CreateEmptySession(ctx context.Context, user agent.UserIdentity, dagName string, safeMode bool) (string, error)
	SendMessage(ctx context.Context, sessionID string, user agent.UserIdentity, req agent.ChatRequest) error
	EnqueueChatMessage(ctx context.Context, sessionID string, user agent.UserIdentity, req agent.ChatRequest) (agent.ChatQueueResult, error)
	FlushQueuedChatMessage(ctx context.Context, sessionID string, user agent.UserIdentity) (agent.ChatQueueResult, error)
	CancelSession(ctx context.Context, sessionID, userID string) error
	SubmitUserResponse(ctx context.Context, sessionID, userID string, resp agent.UserPromptResponse) error
	GenerateAssistantMessage(ctx context.Context, sessionID string, user agent.UserIdentity, dagName, prompt string) (agent.Message, error)
	AppendExternalMessage(ctx context.Context, sessionID string, user agent.UserIdentity, msg agent.Message) (agent.Message, error)
	CompactSessionIfNeeded(ctx context.Context, sessionID string, user agent.UserIdentity) (string, bool, error)
	GetSessionDetail(ctx context.Context, sessionID, userID string) (*agent.StreamResponse, error)
	SubscribeSession(ctx context.Context, sessionID string, user agent.UserIdentity) (agent.StreamResponse, func() (agent.StreamResponse, bool), error)
}

// State stores shared chat bridge session state.
type State struct {
	mu sync.Mutex

	sessionID        string
	ownerUserID      string
	subSessionID     string
	subCancel        context.CancelFunc
	pendingPromptID  string
	lastDeliveredSeq int64
	pendingMessages  []string
	pendingFlushGen  uint64
	working          bool
	hasQueuedInput   bool
}

func (s *State) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

func (s *State) ActiveSession() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID, s.ownerUserID
}

func (s *State) SetActiveSession(sessionID, ownerUserID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionID != sessionID {
		s.lastDeliveredSeq = 0
		s.pendingPromptID = ""
		s.working = false
		s.hasQueuedInput = false
	}
	s.sessionID = sessionID
	s.ownerUserID = ownerUserID
}

// UpdateSessionState stores the latest session-state snapshot from the stream.
func (s *State) UpdateSessionState(sessionState *agent.SessionState) {
	if sessionState == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.working = sessionState.Working
	s.hasQueuedInput = sessionState.HasQueuedUserInput
}

func (s *State) HasQueuedUserInput() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasQueuedInput
}

func (s *State) PendingPromptID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingPromptID
}

func (s *State) SetPendingPrompt(promptID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingPromptID = promptID
}

func (s *State) ClearPendingPrompt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingPromptID = ""
}

func (s *State) LastDeliveredSeq() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastDeliveredSeq
}

func (s *State) MarkDelivered(seq int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq > s.lastDeliveredSeq {
		s.lastDeliveredSeq = seq
	}
}

func (s *State) ClearPendingMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingMessages = nil
	s.pendingFlushGen++
}

func (s *State) EnqueuePendingMessage(text string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingMessages = append(s.pendingMessages, text)
	s.pendingFlushGen++
	return s.pendingFlushGen
}

func (s *State) TakePendingMessages(gen uint64, separator string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.pendingFlushGen || len(s.pendingMessages) == 0 {
		return "", false
	}
	text := strings.Join(append([]string(nil), s.pendingMessages...), separator)
	s.pendingMessages = nil
	return text, true
}

func (s *State) SubscriptionActive(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subSessionID == sessionID && s.subCancel != nil
}

// PrepareSubscription installs a subscription cancel func. When force is false,
// an existing subscription for the same session is reused.
func (s *State) PrepareSubscription(sessionID string, cancel context.CancelFunc, force bool) (cleanup context.CancelFunc, started bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !force && s.subSessionID == sessionID && s.subCancel != nil {
		return cancel, false
	}
	cleanup = s.subCancel
	s.subCancel = cancel
	s.subSessionID = sessionID
	return cleanup, true
}

func (s *State) ClearSessionIfActive(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionID == sessionID {
		s.sessionID = ""
		s.pendingPromptID = ""
		s.working = false
		s.hasQueuedInput = false
	}
}

// Reset clears the shared session state and returns the previous subscription
// cancel function so callers can stop the live subscription outside the lock.
func (s *State) Reset() context.CancelFunc {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel := s.subCancel
	s.subCancel = nil
	s.sessionID = ""
	s.ownerUserID = ""
	s.subSessionID = ""
	s.pendingPromptID = ""
	s.lastDeliveredSeq = 0
	s.pendingMessages = nil
	s.pendingFlushGen++
	s.working = false
	s.hasQueuedInput = false
	return cancel
}

// StreamHandlers contains transport-specific output hooks.
type StreamHandlers struct {
	OnSessionState func(agent.SessionState)
	OnWorking      func(bool)
	OnAssistant    func(agent.Message)
	OnError        func(agent.Message)
	OnPrompt       func(*agent.UserPrompt)
}

// MessageResult describes the outcome of a chat enqueue/flush attempt.
type MessageResult struct {
	SessionID string
	Rotated   bool
	Queued    bool
	Started   bool
	Missing   bool
}

// CompactionResult describes the outcome of a compaction attempt.
type CompactionResult struct {
	SessionID string
	Rotated   bool
	Missing   bool
}

// ProcessStreamResponse handles common delivery bookkeeping and dispatches only
// the transport-specific rendering work to the provided handlers.
func ProcessStreamResponse(state *State, resp agent.StreamResponse, handlers StreamHandlers) {
	if resp.SessionState != nil {
		state.UpdateSessionState(resp.SessionState)
		if handlers.OnSessionState != nil {
			handlers.OnSessionState(*resp.SessionState)
		}
		if handlers.OnWorking != nil {
			handlers.OnWorking(resp.SessionState.Working)
		}
	}

	lastDelivered := state.LastDeliveredSeq()
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
			if msg.Content != "" && handlers.OnAssistant != nil {
				handlers.OnAssistant(msg)
			}
		case agent.MessageTypeError:
			if msg.Content != "" && handlers.OnError != nil {
				handlers.OnError(msg)
			}
		case agent.MessageTypeUserPrompt:
			if msg.UserPrompt != nil && handlers.OnPrompt != nil {
				handlers.OnPrompt(msg.UserPrompt)
			}
		case agent.MessageTypeUser, agent.MessageTypeUIAction:
			// Skip user messages and UI actions in chat bridge output.
		}
	}

	if maxSeen > lastDelivered {
		state.MarkDelivered(maxSeen)
	}
}

// EnqueueMessage appends or merges bot text into the current session.
func EnqueueMessage(ctx context.Context, svc AgentService, state *State, user agent.UserIdentity, req agent.ChatRequest) (MessageResult, error) {
	sessionID := state.SessionID()
	if sessionID == "" {
		return MessageResult{}, nil
	}

	result, err := svc.EnqueueChatMessage(ctx, sessionID, user, req)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			return MessageResult{SessionID: sessionID, Missing: true}, nil
		}
		return MessageResult{SessionID: sessionID}, err
	}
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	if result.Rotated {
		state.SetActiveSession(result.SessionID, user.UserID)
		MarkSessionSnapshotDelivered(ctx, svc, state, result.SessionID, user.UserID)
	}
	return MessageResult{
		SessionID: result.SessionID,
		Rotated:   result.Rotated,
		Queued:    result.Queued,
		Started:   result.Started,
	}, nil
}

// FlushQueuedMessage starts a merged queued bot follow-up turn after the session becomes idle.
func FlushQueuedMessage(ctx context.Context, svc AgentService, state *State, user agent.UserIdentity) (MessageResult, error) {
	sessionID := state.SessionID()
	if sessionID == "" {
		return MessageResult{}, nil
	}

	result, err := svc.FlushQueuedChatMessage(ctx, sessionID, user)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			return MessageResult{SessionID: sessionID, Missing: true}, nil
		}
		return MessageResult{SessionID: sessionID}, err
	}
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	if result.Rotated {
		state.SetActiveSession(result.SessionID, user.UserID)
		MarkSessionSnapshotDelivered(ctx, svc, state, result.SessionID, user.UserID)
	}
	return MessageResult{
		SessionID: result.SessionID,
		Rotated:   result.Rotated,
		Queued:    result.Queued,
		Started:   result.Started,
	}, nil
}

// AppendNotification appends a notification message into the active
// conversation, creating or recreating the session if necessary.
func AppendNotification(ctx context.Context, svc AgentService, state *State, user agent.UserIdentity, dagName string, safeMode bool, msg agent.Message) (string, agent.Message, error) {
	appendToSession := func(sessionID string) (agent.Message, error) {
		return svc.AppendExternalMessage(ctx, sessionID, user, msg)
	}

	sessionID := state.SessionID()
	if sessionID == "" {
		newSessionID, err := svc.CreateEmptySession(ctx, user, dagName, safeMode)
		if err != nil {
			return "", agent.Message{}, err
		}
		state.SetActiveSession(newSessionID, user.UserID)
		stored, err := appendToSession(newSessionID)
		if err != nil {
			return "", agent.Message{}, err
		}
		return newSessionID, stored, nil
	}

	stored, err := appendToSession(sessionID)
	if err == nil {
		return sessionID, stored, nil
	}
	if !errors.Is(err, agent.ErrSessionNotFound) {
		return "", agent.Message{}, err
	}

	newSessionID, err := svc.CreateEmptySession(ctx, user, dagName, safeMode)
	if err != nil {
		return "", agent.Message{}, err
	}
	state.SetActiveSession(newSessionID, user.UserID)
	stored, err = appendToSession(newSessionID)
	if err != nil {
		return "", agent.Message{}, err
	}
	return newSessionID, stored, nil
}

// MaybeCompactSession compacts the current session if needed and seeds delivery
// bookkeeping for the continuation session.
func MaybeCompactSession(ctx context.Context, svc AgentService, state *State, user agent.UserIdentity) (CompactionResult, error) {
	sessionID := state.SessionID()
	if sessionID == "" {
		return CompactionResult{}, nil
	}

	newSessionID, rotated, err := svc.CompactSessionIfNeeded(ctx, sessionID, user)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			return CompactionResult{SessionID: sessionID, Missing: true}, nil
		}
		return CompactionResult{SessionID: sessionID}, err
	}
	if !rotated {
		return CompactionResult{SessionID: newSessionID}, nil
	}

	state.SetActiveSession(newSessionID, user.UserID)
	MarkSessionSnapshotDelivered(ctx, svc, state, newSessionID, user.UserID)
	return CompactionResult{SessionID: newSessionID, Rotated: true}, nil
}

// MarkSessionSnapshotDelivered marks all existing messages in a continuation
// session as already delivered so transports do not replay the handoff summary.
func MarkSessionSnapshotDelivered(ctx context.Context, svc AgentService, state *State, sessionID, userID string) {
	detail, err := svc.GetSessionDetail(ctx, sessionID, userID)
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
		state.MarkDelivered(maxSeq)
	}
}
