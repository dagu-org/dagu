// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
)

func (s *Service) RequestStart(ctx context.Context, name string, req StartRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	if def.Disabled {
		return errors.New("automata is disabled")
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if isService(def) {
		return s.requestServiceStart(ctx, def, state, req)
	}
	if state.State == StateRunning || state.State == StateWaiting || state.State == StatePaused {
		return errors.New("automata already has an active task")
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		instruction = strings.TrimSpace(state.Instruction)
	}
	if instruction == "" {
		return errors.New("instruction is required before starting automata")
	}
	if !hasOpenTask(state.Tasks) {
		return errors.New("at least one open task is required before starting automata")
	}
	if state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
			return err
		}
		if s.sessionStore != nil {
			if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
				return err
			}
		}
	}
	resetRuntimeState(state)
	state.State = StateRunning
	state.Instruction = instruction
	state.InstructionUpdatedAt = s.clock()
	state.InstructionUpdatedBy = req.RequestedBy
	state.StartRequestedAt = s.clock()
	queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
	return s.saveState(ctx, name, state)
}

func (s *Service) requestServiceStart(ctx context.Context, def *Definition, state *State, req StartRequest) error {
	if isServiceActivated(state) {
		return errors.New("service automata is already active")
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		instruction = strings.TrimSpace(state.Instruction)
	}
	if instruction == "" {
		return errors.New("instruction is required before starting automata")
	}
	if state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, def.Name, state.SessionID); err != nil {
			return err
		}
		if s.sessionStore != nil {
			if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
				return err
			}
		}
	}
	resetRuntimeState(state)
	state.State = StateIdle
	state.Instruction = instruction
	state.InstructionUpdatedAt = s.clock()
	state.InstructionUpdatedBy = req.RequestedBy
	state.ActivatedAt = s.clock()
	state.ActivatedBy = req.RequestedBy
	state.WaitingReason = WaitingReasonNone
	if hasOpenTask(state.Tasks) {
		state.State = StateRunning
		state.StartRequestedAt = s.clock()
		queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
	}
	return s.saveState(ctx, def.Name, state)
}

func (s *Service) Pause(ctx context.Context, name, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.State == StatePaused {
		return errors.New("automata is already paused")
	}
	if isService(def) {
		if !isServiceActivated(state) {
			return errors.New("service automata is not active")
		}
	} else if state.State != StateRunning && state.State != StateWaiting {
		return errors.New("only active automata can be paused")
	}
	state.PausedFromState = state.State
	state.State = StatePaused
	state.WaitingReason = WaitingReasonNone
	state.PausedAt = s.clock()
	state.PausedBy = requestedBy
	return s.saveState(ctx, name, state)
}

func (s *Service) Resume(ctx context.Context, name, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.State != StatePaused {
		return errors.New("automata is not paused")
	}
	pausedFromState := state.PausedFromState
	state.PausedAt = time.Time{}
	state.PausedBy = ""
	state.PausedFromState = ""
	if state.PendingPrompt != nil {
		state.State = StateWaiting
		state.WaitingReason = WaitingReasonHuman
		return s.saveState(ctx, name, state)
	}
	if isService(def) && pausedFromState == StateIdle &&
		state.CurrentRunRef == nil && len(state.PendingTurnMessages) == 0 {
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
		return s.saveState(ctx, name, state)
	}
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	if state.CurrentRunRef == nil && len(state.PendingTurnMessages) == 0 {
		queueTurnMessage(state, "resume", s.buildResumeMessage(def, state, requestedBy), s.clock())
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) SubmitHumanResponse(ctx context.Context, name string, req HumanResponseRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.PendingPrompt == nil {
		return errors.New("automata is not waiting for human input")
	}
	if req.PromptID == "" || req.PromptID != state.PendingPrompt.ID {
		return errors.New("prompt ID does not match the pending prompt")
	}
	response := &PromptResponse{
		PromptID:          req.PromptID,
		SelectedOptionIDs: append([]string(nil), req.SelectedOptionIDs...),
		FreeTextResponse:  req.FreeTextResponse,
		RespondedAt:       s.clock(),
	}
	paused := state.State == StatePaused
	queueTurnMessage(state, "human_response", s.buildHumanResponseMessage(state.PendingPrompt, response), s.clock())
	state.PendingPrompt = nil
	state.PendingResponse = nil
	if paused {
		state.WaitingReason = WaitingReasonNone
	} else {
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) CreateTask(ctx context.Context, name string, req CreateTaskRequest) (*Task, error) {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return nil, err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return nil, err
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		return nil, errors.New("task description is required")
	}
	now := s.clock()
	task := Task{
		ID:          uuid.NewString(),
		Description: description,
		State:       TaskStateOpen,
		CreatedAt:   now,
		CreatedBy:   req.RequestedBy,
		UpdatedAt:   now,
		UpdatedBy:   req.RequestedBy,
	}
	state.Tasks = append(state.Tasks, task)
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "created")
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Service) UpdateTask(ctx context.Context, name, taskID string, req UpdateTaskRequest) (*Task, error) {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return nil, err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return nil, err
	}
	idx := findTaskIndex(state.Tasks, taskID)
	if idx < 0 {
		return nil, fmt.Errorf("unknown task %q", taskID)
	}
	if req.Description == nil && req.Done == nil {
		return nil, errors.New("no task changes requested")
	}
	now := s.clock()
	task := &state.Tasks[idx]
	changed := false
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if description == "" {
			return nil, errors.New("task description is required")
		}
		if description != task.Description {
			task.Description = description
			changed = true
		}
	}
	if req.Done != nil {
		targetState := TaskStateOpen
		if *req.Done {
			targetState = TaskStateDone
		}
		if task.State != targetState {
			task.State = targetState
			if targetState == TaskStateDone {
				task.DoneAt = now
				task.DoneBy = req.RequestedBy
			} else {
				task.DoneAt = time.Time{}
				task.DoneBy = ""
			}
			changed = true
		}
	}
	if !changed {
		copied := *task
		return &copied, nil
	}
	task.UpdatedAt = now
	task.UpdatedBy = req.RequestedBy
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "updated")
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	copied := *task
	return &copied, nil
}

func (s *Service) SetTaskDone(ctx context.Context, name, taskID string, done bool, requestedBy string) (*Task, error) {
	return s.UpdateTask(ctx, name, taskID, UpdateTaskRequest{
		Done:        &done,
		RequestedBy: requestedBy,
	})
}

func (s *Service) DeleteTask(ctx context.Context, name, taskID, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	idx := findTaskIndex(state.Tasks, taskID)
	if idx < 0 {
		return fmt.Errorf("unknown task %q", taskID)
	}
	state.Tasks = append(state.Tasks[:idx], state.Tasks[idx+1:]...)
	s.queueTaskListUpdate(ctx, name, state, requestedBy, "deleted")
	return s.saveState(ctx, name, state)
}

func (s *Service) ReorderTasks(ctx context.Context, name string, req ReorderTasksRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if len(req.TaskIDs) != len(state.Tasks) {
		return errors.New("taskIds must contain every task exactly once")
	}
	existing := make(map[string]Task, len(state.Tasks))
	for _, task := range state.Tasks {
		existing[task.ID] = task
	}
	reordered := make([]Task, 0, len(req.TaskIDs))
	seen := make(map[string]struct{}, len(req.TaskIDs))
	for _, taskID := range req.TaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return errors.New("taskIds contains an empty entry")
		}
		if _, ok := seen[taskID]; ok {
			return fmt.Errorf("duplicate task id %q", taskID)
		}
		task, ok := existing[taskID]
		if !ok {
			return fmt.Errorf("unknown task %q", taskID)
		}
		seen[taskID] = struct{}{}
		reordered = append(reordered, task)
	}
	state.Tasks = reordered
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "reordered")
	return s.saveState(ctx, name, state)
}

func (s *Service) SubmitOperatorMessage(ctx context.Context, name string, req OperatorMessageRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if def.Disabled {
		return errors.New("automata is disabled")
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return errors.New("message is required")
	}
	if isService(def) {
		if !isServiceActivated(state) {
			return errors.New("service automata is not active")
		}
		if state.State == StatePaused {
			return errors.New("service automata is paused")
		}
	} else if state.State != StateRunning && state.State != StateWaiting && state.State != StatePaused {
		return errors.New("automata is not running an active task")
	}
	if state.PendingPrompt != nil {
		return errors.New("respond to the pending prompt before sending a general operator message")
	}
	operatorMessage := buildOperatorMessage(req.RequestedBy, message)
	turnMessage := operatorMessage
	if state.SessionID != "" {
		if err := s.appendOperatorMessageToSession(ctx, name, state, operatorMessage); err != nil {
			s.logger.Warn("failed to append queued operator message to automata session",
				"automata", name,
				"session_id", state.SessionID,
				"error", err,
			)
		} else {
			turnMessage = buildOperatorWakeMessage()
		}
	}
	queueTurnMessage(state, "operator_message", turnMessage, s.clock())
	if state.State != StatePaused && state.CurrentRunRef == nil {
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) queueTaskListUpdate(ctx context.Context, name string, state *State, requestedBy, action string) {
	if state == nil {
		return
	}
	if state.State != StateRunning && state.State != StateWaiting && state.State != StatePaused {
		return
	}
	updateMessage := buildTaskListUpdateMessage(requestedBy, action, state.Tasks)
	turnMessage := updateMessage
	if state.SessionID != "" && (state.CurrentRunRef != nil || state.State == StatePaused || state.PendingPrompt != nil) {
		if err := s.appendOperatorMessageToSession(ctx, name, state, updateMessage); err != nil {
			s.logger.Warn("failed to append task list update to automata session",
				"automata", name,
				"session_id", state.SessionID,
				"error", err,
			)
		} else {
			turnMessage = buildOperatorWakeMessage()
		}
	}
	queueTurnMessage(state, "task_list_updated", turnMessage, s.clock())
}

func (s *Service) appendOperatorMessageToSession(ctx context.Context, name string, state *State, content string) error {
	if state == nil || state.SessionID == "" {
		return errors.New("session is not initialized")
	}
	msg := agent.Message{
		ID:        uuid.NewString(),
		SessionID: state.SessionID,
		Type:      agent.MessageTypeUser,
		Content:   content,
		CreatedAt: s.clock(),
		LLMData: &llm.Message{
			Role:    llm.RoleUser,
			Content: content,
		},
	}
	if s.agentAPI != nil {
		if _, err := s.agentAPI.AppendExternalMessage(ctx, state.SessionID, s.systemUser(name), msg); err == nil {
			return nil
		} else if !errors.Is(err, agent.ErrSessionNotFound) {
			return err
		}
	}
	if s.sessionStore == nil {
		return errors.New("session store is not configured")
	}
	seqID, err := s.sessionStore.GetLatestSequenceID(ctx, state.SessionID)
	if err != nil {
		return err
	}
	msg.SequenceID = seqID + 1
	return s.sessionStore.AddMessage(ctx, state.SessionID, &msg)
}

func queueTurnMessage(state *State, kind, message string, now time.Time) {
	if state == nil || strings.TrimSpace(message) == "" {
		return
	}
	state.PendingTurnMessages = append(state.PendingTurnMessages, PendingTurnMessage{
		ID:        uuid.NewString(),
		Kind:      kind,
		Message:   message,
		CreatedAt: now,
	})
}

func buildOperatorMessage(requestedBy, message string) string {
	if requestedBy == "" {
		return "Operator update:\n" + message
	}
	return fmt.Sprintf("Operator update from %s:\n%s", requestedBy, message)
}

func buildTaskListUpdateMessage(requestedBy, action string, tasks []Task) string {
	prefix := "Operator updated the checklist"
	if requestedBy != "" {
		prefix = fmt.Sprintf("Operator %s updated the checklist", requestedBy)
	}
	if strings.TrimSpace(action) != "" {
		prefix = fmt.Sprintf("%s (%s)", prefix, action)
	}
	return fmt.Sprintf("%s.\nCurrent checklist:\n%s", prefix, buildTaskListSummary(tasks))
}

func buildOperatorWakeMessage() string {
	return "A new operator update was appended to the session while execution was blocked. Review the latest user message(s) and continue the task when you can act again."
}
