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
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/google/uuid"
)

var workflowAutomataAllowedTools = []string{
	"read",
	"think",
	"search_skills",
	"use_skill",
	"list_automata_tasks",
	"list_allowed_dags",
	"run_allowed_dag",
	"retry_automata_run",
	"set_automata_task_done",
	"request_human_input",
	"finish_automata",
}

var serviceAutomataAllowedTools = []string{
	"read",
	"think",
	"search_skills",
	"use_skill",
	"list_automata_tasks",
	"list_allowed_dags",
	"run_allowed_dag",
	"retry_automata_run",
	"set_automata_task_done",
	"request_human_input",
}

func (s *Service) ValidateController() error {
	switch {
	case s.agentAPI == nil:
		return errors.New("agent API is not configured")
	case s.subCmdBuilder == nil:
		return errors.New("sub command builder is not configured")
	default:
		return nil
	}
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.ValidateController(); err != nil {
		return err
	}
	ticker := time.NewTicker(s.reconcileEvery)
	defer ticker.Stop()

	if err := s.ReconcileOnce(ctx); err != nil {
		s.logger.Warn("automata reconcile failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.ReconcileOnce(ctx); err != nil {
				s.logger.Warn("automata reconcile failed", "error", err)
			}
		}
	}
}

func (s *Service) ReconcileOnce(ctx context.Context) error {
	defs, err := s.ListDefinitions(ctx)
	if err != nil {
		return err
	}
	for _, def := range defs {
		if err := s.reconcileDefinition(ctx, def); err != nil {
			s.logger.Warn("automata reconcile failed", "automata", def.Name, "error", err)
		}
	}
	return nil
}

func (s *Service) reconcileDefinition(ctx context.Context, def *Definition) error {
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if def.Disabled {
		return nil
	}
	if state.State == StatePaused {
		return s.reconcilePausedDefinition(ctx, def, state)
	}

	if state.PendingPrompt != nil && state.PendingResponse != nil {
		message := s.buildHumanResponseMessage(state.PendingPrompt, state.PendingResponse)
		queueTurnMessage(state, "human_response", message, s.clock())
		state.PendingPrompt = nil
		state.PendingResponse = nil
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return err
		}
	}

	if state.CurrentRunRef != nil {
		return s.reconcileCurrentRun(ctx, def, state)
	}

	if state.PendingPrompt != nil {
		if state.State != StateWaiting || state.WaitingReason != WaitingReasonHuman {
			state.State = StateWaiting
			state.WaitingReason = WaitingReasonHuman
			return s.saveState(ctx, def.Name, state)
		}
		return nil
	}

	if state.State != StateRunning {
		return nil
	}

	if len(state.PendingTurnMessages) > 0 {
		return s.flushPendingTurnMessages(ctx, def, state)
	}

	if flushed, err := s.flushQueuedSessionTurn(ctx, def.Name, state); err != nil {
		return s.recordControllerError(ctx, def, state, "flush_queued_turn_failed", err)
	} else if flushed {
		state.LastError = ""
		return s.saveState(ctx, def.Name, state)
	}

	// A queued follow-up inside the backing chat session is not enough to keep
	// the Automata in `running` once this reconcile pass failed to start it.
	activity := s.inspectSessionActivity(ctx, def.Name, state)
	if !activity.Working && !activity.HasPendingPrompt {
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
		return s.saveState(ctx, def.Name, state)
	}

	return nil
}

func (s *Service) reconcilePausedDefinition(ctx context.Context, def *Definition, state *State) error {
	if s.agentAPI == nil || state.SessionID == "" {
		return nil
	}
	err := s.agentAPI.CancelSession(ctx, state.SessionID, s.systemUser(def.Name).UserID)
	if err == nil || errors.Is(err, agent.ErrSessionNotFound) {
		return nil
	}
	s.logger.Warn("automata pause cancel failed", "automata", def.Name, "error", err)
	return nil
}

func (s *Service) reconcileCurrentRun(ctx context.Context, def *Definition, state *State) error {
	statuses, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithFrom(exec.NewUTC(time.Unix(0, 0))),
		exec.WithExactName(state.CurrentRunRef.Name),
		exec.WithDAGRunID(state.CurrentRunRef.ID),
		exec.WithLimit(1),
	)
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		prevError := state.LastError
		state.LastError = fmt.Sprintf("child DAG run %s not found", state.CurrentRunRef.String())
		state.LastRunRef = state.CurrentRunRef
		state.CurrentRunRef = nil
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
		queueTurnMessage(state, "child_run_missing", fmt.Sprintf(
			"Tracked child DAG run %s could not be found. Investigate the missing run and decide the next action.",
			state.LastRunRef.String(),
		), s.clock())
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return err
		}
		if state.LastError != prevError {
			s.eventEmitter().error(ctx, def, state, "child_run_missing", s.clock())
		}
		return s.flushPendingTurnMessages(ctx, def, state)
	}
	status := statuses[0]
	if status.Status.IsWaiting() {
		if state.State != StateWaiting || state.WaitingReason != WaitingReasonDAG {
			state.State = StateWaiting
			state.WaitingReason = WaitingReasonDAG
			return s.saveState(ctx, def.Name, state)
		}
		return nil
	}
	if status.Status.IsActive() {
		if state.State != StateRunning {
			state.State = StateRunning
			state.WaitingReason = WaitingReasonNone
			return s.saveState(ctx, def.Name, state)
		}
		return nil
	}

	state.LastRunRef = state.CurrentRunRef
	state.CurrentRunRef = nil
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	state.LastSummary = summarizeRunStatus(status)
	if status.Error != "" {
		state.LastError = status.Error
	} else {
		state.LastError = ""
	}
	queueTurnMessage(state, "child_run_complete", s.buildRunCompletionMessage(status), s.clock())
	if err := s.saveState(ctx, def.Name, state); err != nil {
		return err
	}
	return s.flushPendingTurnMessages(ctx, def, state)
}

func (s *Service) flushPendingTurnMessages(ctx context.Context, def *Definition, state *State) error {
	if len(state.PendingTurnMessages) == 0 {
		return nil
	}
	message := buildPendingTurnMessageText(state.PendingTurnMessages)
	return s.startTurn(ctx, def, state, message)
}

func (s *Service) startTurn(ctx context.Context, def *Definition, state *State, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	runtimeOpts, err := s.runtimeOptions(ctx, def, state)
	if err != nil {
		return s.recordControllerError(ctx, def, state, "runtime_options_failed", err)
	}
	user := s.systemUser(def.Name)
	if state.SessionID == "" {
		sessionID, err := s.agentAPI.CreateEmptySessionWithRuntime(ctx, user, "", def.Agent.SafeMode, runtimeOpts)
		if err != nil {
			return s.recordControllerError(ctx, def, state, "session_create_failed", err)
		}
		state.SessionID = sessionID
	}

	result, err := s.agentAPI.EnqueueChatMessageWithRuntime(ctx, state.SessionID, user, agent.ChatRequest{
		Message:  message,
		Model:    def.Agent.Model,
		SafeMode: def.Agent.SafeMode,
	}, runtimeOpts)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			sessionID, createErr := s.agentAPI.CreateEmptySessionWithRuntime(ctx, user, "", def.Agent.SafeMode, runtimeOpts)
			if createErr != nil {
				return s.recordControllerError(ctx, def, state, "session_recreate_failed", createErr)
			}
			state.SessionID = sessionID
			result, err = s.agentAPI.EnqueueChatMessageWithRuntime(ctx, state.SessionID, user, agent.ChatRequest{
				Message:  message,
				Model:    def.Agent.Model,
				SafeMode: def.Agent.SafeMode,
			}, runtimeOpts)
		}
	}
	if err != nil {
		return s.recordControllerError(ctx, def, state, "enqueue_turn_failed", err)
	}
	if result.SessionID != "" {
		state.SessionID = result.SessionID
	}
	if result.Queued {
		if _, flushErr := s.flushQueuedSessionTurn(ctx, def.Name, state); flushErr != nil {
			return s.recordControllerError(ctx, def, state, "flush_queued_turn_failed", flushErr)
		}
	}

	state.LastError = ""
	state.State = StateRunning
	state.PendingTurnMessages = nil
	state.StartRequestedAt = time.Time{}
	state.LastTriggeredAt = s.clock()
	state.WaitingReason = WaitingReasonNone
	return s.saveState(ctx, def.Name, state)
}

func (s *Service) recordControllerError(ctx context.Context, def *Definition, state *State, reason string, err error) error {
	if err == nil {
		return nil
	}
	prevError := state.LastError
	state.LastError = err.Error()
	if saveErr := s.saveState(ctx, def.Name, state); saveErr != nil {
		return saveErr
	}
	if state.LastError != prevError {
		s.eventEmitter().error(ctx, def, state, reason, s.clock())
	}
	return nil
}

func (s *Service) flushQueuedSessionTurn(ctx context.Context, name string, state *State) (bool, error) {
	if s.agentAPI == nil || state == nil || state.SessionID == "" {
		return false, nil
	}
	result, err := s.agentAPI.FlushQueuedChatMessage(ctx, state.SessionID, s.systemUser(name))
	if err != nil {
		return false, err
	}
	changed := false
	if result.SessionID != "" && result.SessionID != state.SessionID {
		state.SessionID = result.SessionID
		changed = true
	}
	if result.Started {
		state.LastTriggeredAt = s.clock()
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
		changed = true
	}
	return changed, nil
}

type sessionActivity struct {
	Working          bool
	HasPendingPrompt bool
	HasQueuedInput   bool
}

func (s *Service) inspectSessionActivity(ctx context.Context, name string, state *State) sessionActivity {
	if s.agentAPI == nil || state == nil || state.SessionID == "" {
		return sessionActivity{}
	}
	detail, err := s.agentAPI.GetSessionDetail(ctx, state.SessionID, s.systemUser(name).UserID)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			return sessionActivity{}
		}
		s.logger.Warn("failed to inspect automata session state",
			"automata", name,
			"session_id", state.SessionID,
			"error", err,
		)
		return sessionActivity{Working: true}
	}
	if detail == nil || detail.SessionState == nil {
		return sessionActivity{}
	}
	return sessionActivity{
		Working:          detail.SessionState.Working,
		HasPendingPrompt: detail.SessionState.HasPendingPrompt,
		HasQueuedInput:   detail.SessionState.HasQueuedUserInput,
	}
}

func (s *Service) runtimeOptions(ctx context.Context, def *Definition, state *State) (*agent.SessionRuntimeOptions, error) {
	allowedDAGs, err := s.resolveAllowedDAGSet(ctx, def.AllowedDAGs)
	if err != nil {
		return nil, err
	}
	var soul *agent.Soul
	if def.Agent.Soul != "" && s.soulStore != nil {
		loaded, err := s.soulStore.GetByID(ctx, def.Agent.Soul)
		if err != nil {
			return nil, err
		}
		soul = loaded
	}
	return &agent.SessionRuntimeOptions{
		Model:             def.Agent.Model,
		AllowedTools:      allowedToolsForDefinition(def),
		SystemPromptExtra: s.buildSystemPromptExtra(def, state, allowedDAGs),
		EnabledSkills:     append([]string(nil), def.Agent.EnabledSkills...),
		Soul:              soul,
		AllowClearSoul:    def.Agent.Soul == "",
		AutomataName:      def.Name,
		AutomataRuntime: &controllerRuntime{
			service: s,
			def:     def,
			state:   state,
		},
	}, nil
}

func allowedToolsForDefinition(def *Definition) []string {
	if isService(def) {
		return append([]string(nil), serviceAutomataAllowedTools...)
	}
	return append([]string(nil), workflowAutomataAllowedTools...)
}

func (s *Service) buildSystemPromptExtra(def *Definition, state *State, allowed []AllowedDAGInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "You are controlling Automata %q.\n", def.Name)
	fmt.Fprintf(&sb, "Kind: %s\n", def.Kind)
	if goal := strings.TrimSpace(def.Goal); goal != "" {
		fmt.Fprintf(&sb, "Goal: %s\n", goal)
	} else {
		sb.WriteString("Goal: none provided yet.\n")
	}
	if instruction := strings.TrimSpace(state.Instruction); instruction != "" {
		fmt.Fprintf(&sb, "Current instruction: %s\n", instruction)
	} else {
		sb.WriteString("Current instruction: none provided yet.\n")
	}
	if def.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", def.Description)
	}
	fmt.Fprintf(&sb, "Lifecycle state: %s\n", state.State)
	sb.WriteString("Task list:\n")
	sb.WriteString(buildTaskListSummary(state.Tasks))
	sb.WriteString("\n")
	sb.WriteString("Allowed DAGs:\n")
	if len(allowed) == 0 {
		sb.WriteString("- none\n")
	} else {
		for _, dag := range allowed {
			fmt.Fprintf(&sb, "- %s", dag.Name)
			if dag.Description != "" {
				fmt.Fprintf(&sb, ": %s", dag.Description)
			}
			if len(dag.Tags) > 0 {
				fmt.Fprintf(&sb, " [tags: %s]", strings.Join(dag.Tags, ", "))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("Rules:\n")
	sb.WriteString("- Use only the tools available in this session.\n")
	sb.WriteString("- The task list is operator-owned. You may only mark existing tasks done or open again.\n")
	sb.WriteString("- Open tasks are not strictly ordered. Choose whichever open task best fits the current context unless the operator gave explicit priority.\n")
	sb.WriteString("- Do not create, edit, reorder, or delete task list items.\n")
	sb.WriteString("- Use list_automata_tasks when you need a fresh view of the task list.\n")
	sb.WriteString("- Use set_automata_task_done to mark an existing task done or reopen it if more work is needed.\n")
	sb.WriteString("- Use run_allowed_dag for execution and wait for the scheduler to resume you.\n")
	sb.WriteString("- Use request_human_input if blocked on approval or clarification.\n")
	if isService(def) {
		sb.WriteString("- This is a persistent service. Do not finish it when current work is done.\n")
		sb.WriteString("- If there is nothing actionable right now, return to standby and wait for the next schedule tick or operator message.\n")
	} else {
		sb.WriteString("- Use finish_automata only when the goal is complete.\n")
	}
	sb.WriteString("- Do not ask for shell commands, file edits, or tools you do not have.\n")
	return sb.String()
}

func (s *Service) buildKickoffMessage(def *Definition, state *State) string {
	if isService(def) {
		return fmt.Sprintf(
			"Activate service Automata %q. Current instruction: %q. Review the open tasks and current context. Continue any actionable work, choosing whichever open task is most appropriate. If work must be executed, run one allowlisted DAG. If blocked, request human input. If there is nothing actionable right now, return to standby without finishing the automata.",
			def.Name,
			state.Instruction,
		)
	}
	if strings.TrimSpace(def.Goal) == "" {
		return fmt.Sprintf(
			"Continue Automata %q. Current instruction: %q. Review the open tasks and current context, then choose the most appropriate work. If work must be executed, run one allowlisted DAG. If blocked, request human input. If complete, finish the automata.",
			def.Name,
			state.Instruction,
		)
	}
	return fmt.Sprintf(
		"Continue Automata %q. Current instruction: %q. Review the open tasks and choose the most appropriate work toward the goal. If work must be executed, run one allowlisted DAG. If blocked, request human input. If complete, finish the automata.",
		def.Name,
		state.Instruction,
	)
}

func (s *Service) buildResumeMessage(def *Definition, state *State, requestedBy string) string {
	if isService(def) {
		if requestedBy == "" {
			return fmt.Sprintf(
				"Service Automata %q was resumed. Current instruction: %q. Continue from the latest context and choose any appropriate open task if more work is needed. If there is nothing actionable right now, return to standby.",
				def.Name,
				state.Instruction,
			)
		}
		return fmt.Sprintf(
			"Service Automata %q was resumed by %s. Current instruction: %q. Continue from the latest context and choose any appropriate open task if more work is needed. If there is nothing actionable right now, return to standby.",
			def.Name,
			requestedBy,
			state.Instruction,
		)
	}
	if requestedBy == "" {
		return fmt.Sprintf(
			"Automata %q was resumed. Current instruction: %q. Continue from the latest context and choose any appropriate open task.",
			def.Name,
			state.Instruction,
		)
	}
	return fmt.Sprintf(
		"Automata %q was resumed by %s. Current instruction: %q. Continue from the latest context and choose any appropriate open task.",
		def.Name,
		requestedBy,
		state.Instruction,
	)
}

func (s *Service) buildRunCompletionMessage(status *exec.DAGRunStatus) string {
	return fmt.Sprintf(
		"Child DAG run completed.\nDAG: %s\nRun ID: %s\nStatus: %s\nError: %s\nSummary: %s\nDecide the next action.",
		status.Name,
		status.DAGRunID,
		status.Status.String(),
		status.Error,
		summarizeRunStatus(status),
	)
}

func (s *Service) buildHumanResponseMessage(prompt *Prompt, response *PromptResponse) string {
	return fmt.Sprintf(
		"The user responded to your prompt %q.\nSelected options: %s\nFree text: %s\nContinue the automata.",
		prompt.Question,
		strings.Join(response.SelectedOptionIDs, ", "),
		response.FreeTextResponse,
	)
}

func (s *Service) buildScheduledTickMessage(def *Definition, state *State, tickTime time.Time) string {
	return fmt.Sprintf(
		"Scheduled wake-up for service Automata %q at %s. Current instruction: %q. Review the open tasks and current context. If there is actionable work, continue it or run one allowlisted DAG. Choose whichever open task is most appropriate. If there is nothing actionable right now, return to standby without finishing the automata.",
		def.Name,
		tickTime.Format(time.RFC3339),
		state.Instruction,
	)
}

type controllerRuntime struct {
	service *Service
	def     *Definition
	state   *State
}

func (r *controllerRuntime) ListTasks(_ context.Context) ([]agent.AutomataTask, error) {
	out := make([]agent.AutomataTask, 0, len(r.state.Tasks))
	for _, task := range r.state.Tasks {
		out = append(out, agent.AutomataTask{
			ID:          task.ID,
			Description: task.Description,
			State:       string(task.State),
		})
	}
	return out, nil
}

func (r *controllerRuntime) ListAllowedDAGs(ctx context.Context) ([]agent.AutomataAllowedDAG, error) {
	items, err := r.service.resolveAllowedDAGSet(ctx, r.def.AllowedDAGs)
	if err != nil {
		return nil, err
	}
	out := make([]agent.AutomataAllowedDAG, 0, len(items))
	for _, item := range items {
		out = append(out, agent.AutomataAllowedDAG{
			Name:        item.Name,
			Description: item.Description,
			Tags:        item.Tags,
		})
	}
	return out, nil
}

func (r *controllerRuntime) RunAllowedDAG(ctx context.Context, input agent.AutomataRunDAGInput) (agent.AutomataRunDAGResult, error) {
	if r.state.CurrentRunRef != nil {
		return agent.AutomataRunDAGResult{}, fmt.Errorf("a child DAG run is already active")
	}
	if r.state.PendingPrompt != nil {
		return agent.AutomataRunDAGResult{}, fmt.Errorf("cannot start a child DAG while waiting for human input")
	}
	allowed, err := r.service.resolveAllowedDAGSet(ctx, r.def.AllowedDAGs)
	if err != nil {
		return agent.AutomataRunDAGResult{}, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[item.Name] = struct{}{}
	}
	if _, ok := allowedSet[input.DAGName]; !ok {
		return agent.AutomataRunDAGResult{}, fmt.Errorf("DAG %q is not allowlisted", input.DAGName)
	}
	dag, err := r.service.dagStore.GetDetails(ctx, input.DAGName)
	if err != nil {
		return agent.AutomataRunDAGResult{}, err
	}
	if r.state.CurrentCycleID == "" {
		r.state.CurrentCycleID = nextCycleID()
	}
	runID := fmt.Sprintf("automata-%d", r.service.clock().UnixNano())
	tags := []string{
		fmt.Sprintf("automata=%s", strings.ToLower(r.def.Name)),
		fmt.Sprintf("automata_cycle=%s", r.state.CurrentCycleID),
	}
	if len(r.def.Tags) > 0 {
		tags = append(tags, r.def.Tags...)
	}
	tagText := strings.Join(tags, ",")
	spec := r.service.subCmdBuilder.Enqueue(dag, runtime.EnqueueOptions{
		Quiet:       true,
		DAGRunID:    runID,
		Params:      input.Params,
		TriggerType: core.TriggerTypeAutomata.String(),
		Tags:        tagText,
	})
	if err := runtime.Run(ctx, spec); err != nil {
		return agent.AutomataRunDAGResult{}, err
	}
	ref := exec.NewDAGRunRef(dag.Name, runID)
	r.state.CurrentRunRef = &ref
	r.state.State = StateRunning
	r.state.WaitingReason = WaitingReasonNone
	if err := r.service.saveState(ctx, r.def.Name, r.state); err != nil {
		return agent.AutomataRunDAGResult{}, err
	}
	return agent.AutomataRunDAGResult{DAGName: dag.Name, DAGRunID: runID}, nil
}

func (r *controllerRuntime) RetryCurrentRun(ctx context.Context) (agent.AutomataRunDAGResult, error) {
	if r.state.LastRunRef == nil {
		return agent.AutomataRunDAGResult{}, fmt.Errorf("no prior DAG run available to retry")
	}
	return r.RunAllowedDAG(ctx, agent.AutomataRunDAGInput{DAGName: r.state.LastRunRef.Name})
}

func (r *controllerRuntime) SetTaskDone(ctx context.Context, taskID string, done bool) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id is required")
	}
	_, err := r.service.SetTaskDone(ctx, r.def.Name, taskID, done, "agent")
	return err
}

func (r *controllerRuntime) RequestHumanInput(ctx context.Context, prompt agent.AutomataHumanPrompt) error {
	if strings.TrimSpace(prompt.Question) == "" {
		return fmt.Errorf("question is required")
	}
	if r.state.CurrentRunRef != nil {
		return fmt.Errorf("cannot request human input while a child DAG run is active")
	}
	if r.state.PendingPrompt != nil {
		return fmt.Errorf("automata is already waiting for human input")
	}
	r.state.PendingPrompt = &Prompt{
		ID:                  uuid.NewString(),
		Question:            prompt.Question,
		Options:             append([]agent.UserPromptOption(nil), prompt.Options...),
		AllowFreeText:       prompt.AllowFreeText,
		FreeTextPlaceholder: prompt.FreeTextPlaceholder,
		CreatedAt:           r.service.clock(),
	}
	r.state.PendingResponse = nil
	r.state.State = StateWaiting
	r.state.WaitingReason = WaitingReasonHuman
	if err := r.service.saveState(ctx, r.def.Name, r.state); err != nil {
		return err
	}
	r.service.eventEmitter().needsInput(ctx, r.def, r.state)
	return nil
}

func (r *controllerRuntime) Finish(ctx context.Context, summary string) error {
	if isService(r.def) {
		return fmt.Errorf("cannot finish a service automata")
	}
	if r.state.CurrentRunRef != nil {
		return fmt.Errorf("cannot finish automata while a child DAG run is active")
	}
	r.state.State = StateFinished
	r.state.WaitingReason = WaitingReasonNone
	r.state.PendingPrompt = nil
	r.state.PendingResponse = nil
	r.state.PendingTurnMessages = nil
	r.state.CurrentRunRef = nil
	if r.state.CurrentCycleID == "" {
		r.state.CurrentCycleID = nextCycleID()
	}
	r.state.FinishedAt = r.service.clock()
	r.state.LastSummary = summary
	if err := r.service.saveState(ctx, r.def.Name, r.state); err != nil {
		return err
	}
	r.service.eventEmitter().finished(ctx, r.def, r.state)
	return nil
}

func summarizeRunStatus(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	if status.Error != "" {
		return status.Error
	}
	var parts []string
	for _, err := range status.Errors() {
		parts = append(parts, err.Error())
	}
	if len(parts) == 0 {
		return status.Status.String()
	}
	return strings.Join(parts, "; ")
}

func buildPendingTurnMessageText(messages []PendingTurnMessage) string {
	if len(messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, message := range messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(message.Message)
	}
	return sb.String()
}
