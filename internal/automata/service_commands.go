// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/llm"
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
	if !hasTaskTemplates(state.TaskTemplates) {
		return errors.New("at least one task template is required before starting automata")
	}
	if err := s.startWorkflowCycle(ctx, def, state, instruction, req.RequestedBy); err != nil {
		return err
	}
	queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
	return s.saveState(ctx, name, state)
}

func (s *Service) requestServiceStart(ctx context.Context, def *Definition, state *State, req StartRequest) error {
	if isServiceActivated(state) {
		return errors.New("service automata is already active")
	}
	instruction := strings.TrimSpace(def.StandingInstruction)
	if instruction == "" {
		return errors.New("standing instruction is required before activating service automata")
	}
	if !hasTaskTemplates(state.TaskTemplates) {
		return errors.New("at least one task template is required before activating service automata")
	}
	if err := s.startServiceCycle(ctx, def, state, instruction, req.RequestedBy); err != nil {
		return err
	}
	state.ActivatedAt = s.clock()
	state.ActivatedBy = req.RequestedBy
	queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
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
	task := TaskTemplate{
		ID:          uuid.NewString(),
		Description: description,
		CreatedAt:   now,
		CreatedBy:   req.RequestedBy,
		UpdatedAt:   now,
		UpdatedBy:   req.RequestedBy,
	}
	state.TaskTemplates = append(state.TaskTemplates, task)
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	resp := taskFromTemplate(task)
	return &resp, nil
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
	templateIdx := findTaskTemplateIndex(state.TaskTemplates, taskID)
	if templateIdx < 0 {
		return nil, fmt.Errorf("unknown task %q", taskID)
	}
	currentIdx := findTaskIndex(state.Tasks, taskID)
	if req.Description == nil && req.Done == nil {
		return nil, errors.New("no task changes requested")
	}
	now := s.clock()
	changed := false
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if description == "" {
			return nil, errors.New("task description is required")
		}
		template := &state.TaskTemplates[templateIdx]
		if description != template.Description {
			template.Description = description
			template.UpdatedAt = now
			template.UpdatedBy = req.RequestedBy
			changed = true
		}
		if currentIdx >= 0 && state.Tasks[currentIdx].Description != description {
			state.Tasks[currentIdx].Description = description
			state.Tasks[currentIdx].UpdatedAt = now
			state.Tasks[currentIdx].UpdatedBy = req.RequestedBy
			changed = true
		}
	}
	if req.Done != nil {
		if currentIdx < 0 {
			return nil, errors.New("task state can only be updated for the current cycle")
		}
		targetState := TaskStateOpen
		if *req.Done {
			targetState = TaskStateDone
		}
		task := &state.Tasks[currentIdx]
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
			s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "updated")
		}
	}
	if !changed {
		if currentIdx >= 0 {
			copied := state.Tasks[currentIdx]
			return &copied, nil
		}
		copied := taskFromTemplate(state.TaskTemplates[templateIdx])
		return &copied, nil
	}
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	if currentIdx >= 0 {
		copied := state.Tasks[currentIdx]
		return &copied, nil
	}
	copied := taskFromTemplate(state.TaskTemplates[templateIdx])
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
	idx := findTaskTemplateIndex(state.TaskTemplates, taskID)
	if idx < 0 {
		return fmt.Errorf("unknown task %q", taskID)
	}
	state.TaskTemplates = append(state.TaskTemplates[:idx], state.TaskTemplates[idx+1:]...)
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
	if len(req.TaskIDs) != len(state.TaskTemplates) {
		return errors.New("taskIds must contain every task exactly once")
	}
	existing := make(map[string]TaskTemplate, len(state.TaskTemplates))
	for _, task := range state.TaskTemplates {
		existing[task.ID] = task
	}
	reordered := make([]TaskTemplate, 0, len(req.TaskIDs))
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
	state.TaskTemplates = reordered
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
		if state.State == StateIdle {
			if !hasTaskTemplates(state.TaskTemplates) {
				return errors.New("at least one task template is required before starting service automata cycle")
			}
			instruction := strings.TrimSpace(def.StandingInstruction)
			if instruction == "" {
				return errors.New("standing instruction is required before starting service automata cycle")
			}
			if err := s.startServiceCycle(ctx, def, state, instruction, "operator"); err != nil {
				return err
			}
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

func (s *Service) startWorkflowCycle(ctx context.Context, def *Definition, state *State, instruction, requestedBy string) error {
	if err := s.cleanupRuntime(ctx, def.Name, true); err != nil {
		return err
	}
	resetRuntimeState(state)
	now := s.clock()
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, now)
	state.CurrentCycleID = nextCycleID()
	state.LastTriggeredAt = now
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	state.Instruction = instruction
	state.InstructionUpdatedAt = now
	state.InstructionUpdatedBy = requestedBy
	state.StartRequestedAt = now
	return nil
}

func (s *Service) startServiceCycle(ctx context.Context, def *Definition, state *State, instruction, requestedBy string) error {
	if err := s.cleanupRuntime(ctx, def.Name, true); err != nil {
		return err
	}
	activatedAt := state.ActivatedAt
	activatedBy := state.ActivatedBy
	clearCurrentCycleState(state)
	now := s.clock()
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, now)
	state.CurrentCycleID = nextCycleID()
	state.LastTriggeredAt = now
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	state.Instruction = instruction
	state.InstructionUpdatedAt = now
	state.InstructionUpdatedBy = requestedBy
	state.StartRequestedAt = now
	state.ActivatedAt = firstNonZeroTime(activatedAt, now)
	if activatedBy != "" {
		state.ActivatedBy = activatedBy
	} else {
		state.ActivatedBy = requestedBy
	}
	return nil
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
	prefix := "Operator updated the task list"
	if requestedBy != "" {
		prefix = fmt.Sprintf("Operator %s updated the task list", requestedBy)
	}
	if strings.TrimSpace(action) != "" {
		prefix = fmt.Sprintf("%s (%s)", prefix, action)
	}
	return fmt.Sprintf("%s.\nCurrent task list:\n%s", prefix, buildTaskListSummary(tasks))
}

func buildOperatorWakeMessage() string {
	return "A new operator update was appended to the session while execution was blocked. Review the latest user message(s) and continue the task when you can act again."
}
