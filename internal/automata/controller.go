// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/google/uuid"
)

var automataAllowedTools = []string{
	"read",
	"think",
	"search_skills",
	"use_skill",
	"list_allowed_dags",
	"run_allowed_dag",
	"retry_automata_run",
	"set_automata_stage",
	"request_human_input",
	"finish_automata",
}

func (s *Service) Run(ctx context.Context) {
	if s.agentAPI == nil || s.subCmdBuilder == nil {
		s.logger.Info("automata controller disabled", "reason", "runtime not configured")
		return
	}
	ticker := time.NewTicker(s.reconcileEvery)
	defer ticker.Stop()

	if err := s.ReconcileOnce(ctx); err != nil {
		s.logger.Warn("automata reconcile failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
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

	if state.PendingPrompt != nil && state.PendingResponse != nil {
		message := s.buildHumanResponseMessage(state.PendingPrompt, state.PendingResponse)
		state.PendingPrompt = nil
		state.PendingResponse = nil
		state.WaitingReason = WaitingReasonNone
		return s.startTurn(ctx, def, state, message)
	}

	if state.CurrentRunRef != nil {
		return s.reconcileCurrentRun(ctx, def, state)
	}

	if state.State == StateRunning {
		detail, err := s.agentAPI.GetSessionDetail(ctx, state.SessionID, s.systemUser(def.Name).UserID)
		if err == nil && detail != nil && detail.SessionState != nil && detail.SessionState.Working {
			return nil
		}
		state.State = StateIdle
		return s.saveState(ctx, def.Name, state)
	}

	if state.PendingPrompt != nil {
		if state.State != StateWaiting || state.WaitingReason != WaitingReasonHuman {
			state.State = StateWaiting
			state.WaitingReason = WaitingReasonHuman
			return s.saveState(ctx, def.Name, state)
		}
		return nil
	}

	if !state.StartRequestedAt.IsZero() {
		return s.startTurn(ctx, def, state, s.buildKickoffMessage(def, state))
	}

	if s.shouldStartScheduled(def, state) {
		return s.startTurn(ctx, def, state, s.buildKickoffMessage(def, state))
	}

	return nil
}

func (s *Service) shouldStartScheduled(def *Definition, state *State) bool {
	if len(def.Schedule) == 0 || state.State != StateIdle {
		return false
	}
	loc := s.cfg.Core.Location
	if loc == nil {
		loc = time.Local
	}
	now := s.clock().In(loc)
	minute := now.Truncate(time.Minute)
	if !state.LastScheduleMinute.IsZero() && !minute.After(state.LastScheduleMinute) {
		return false
	}
	prev := minute.Add(-time.Minute)
	for _, sched := range def.Schedule {
		if sched.Parsed == nil {
			continue
		}
		if sched.Parsed.Next(prev) == minute {
			state.LastScheduleMinute = minute
			return true
		}
	}
	return false
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
		state.LastError = fmt.Sprintf("child DAG run %s not found", state.CurrentRunRef.String())
		state.CurrentRunRef = nil
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
		return s.saveState(ctx, def.Name, state)
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
	return s.startTurn(ctx, def, state, s.buildRunCompletionMessage(status))
}

func (s *Service) startTurn(ctx context.Context, def *Definition, state *State, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	runtimeOpts, err := s.runtimeOptions(ctx, def, state)
	if err != nil {
		return err
	}
	user := s.systemUser(def.Name)
	if state.SessionID == "" {
		sessionID, err := s.agentAPI.CreateEmptySessionWithRuntime(ctx, user, "", def.Agent.SafeMode, runtimeOpts)
		if err != nil {
			return err
		}
		state.SessionID = sessionID
	}

	_, err = s.agentAPI.EnqueueChatMessageWithRuntime(ctx, state.SessionID, user, agent.ChatRequest{
		Message:  message,
		Model:    def.Agent.Model,
		SafeMode: def.Agent.SafeMode,
	}, runtimeOpts)
	if err != nil {
		if errors.Is(err, agent.ErrSessionNotFound) {
			sessionID, createErr := s.agentAPI.CreateEmptySessionWithRuntime(ctx, user, "", def.Agent.SafeMode, runtimeOpts)
			if createErr != nil {
				state.LastError = createErr.Error()
				return s.saveState(ctx, def.Name, state)
			}
			state.SessionID = sessionID
			_, err = s.agentAPI.EnqueueChatMessageWithRuntime(ctx, state.SessionID, user, agent.ChatRequest{
				Message:  message,
				Model:    def.Agent.Model,
				SafeMode: def.Agent.SafeMode,
			}, runtimeOpts)
		}
	}
	if err != nil {
		state.LastError = err.Error()
		return s.saveState(ctx, def.Name, state)
	}

	state.State = StateRunning
	state.StartRequestedAt = time.Time{}
	state.LastTriggeredAt = s.clock()
	state.WaitingReason = WaitingReasonNone
	return s.saveState(ctx, def.Name, state)
}

func (s *Service) runtimeOptions(ctx context.Context, def *Definition, state *State) (*agent.SessionRuntimeOptions, error) {
	allowedDAGs, err := s.resolveAllowedDAGs(ctx, def)
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
		AllowedTools:      append([]string(nil), automataAllowedTools...),
		SystemPromptExtra: s.buildSystemPromptExtra(def, state, allowedDAGs),
		EnabledSkills:     append([]string(nil), def.Agent.EnabledSkills...),
		Soul:              soul,
		AllowClearSoul:    def.Agent.Soul == "",
		AutomataRuntime: &controllerRuntime{
			service: s,
			def:     def,
			state:   state,
		},
	}, nil
}

func (s *Service) buildSystemPromptExtra(def *Definition, state *State, allowed []AllowedDAGInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "You are controlling Automata %q.\n", def.Name)
	fmt.Fprintf(&sb, "Purpose: %s\n", def.Purpose)
	fmt.Fprintf(&sb, "Goal: %s\n", def.Goal)
	if def.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", def.Description)
	}
	fmt.Fprintf(&sb, "Lifecycle state: %s\n", state.State)
	fmt.Fprintf(&sb, "Current stage: %s\n", state.CurrentStage)
	fmt.Fprintf(&sb, "Available stages: %s\n", strings.Join(def.Stages, ", "))
	sb.WriteString("Allowed DAGs:\n")
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
	sb.WriteString("Rules:\n")
	sb.WriteString("- Use only the tools available in this session.\n")
	sb.WriteString("- Use set_automata_stage when the workflow stage changes.\n")
	sb.WriteString("- Use run_allowed_dag for execution and wait for the scheduler to resume you.\n")
	sb.WriteString("- Use request_human_input if blocked on approval or clarification.\n")
	sb.WriteString("- Use finish_automata only when the goal is complete.\n")
	sb.WriteString("- Do not ask for shell commands, file edits, or tools you do not have.\n")
	return sb.String()
}

func (s *Service) buildKickoffMessage(def *Definition, state *State) string {
	return fmt.Sprintf(
		"Continue Automata %q. Current stage: %q. Decide the next best action toward the goal. If work must be executed, run one allowlisted DAG. If blocked, request human input. If complete, finish the automata.",
		def.Name,
		state.CurrentStage,
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

type controllerRuntime struct {
	service *Service
	def     *Definition
	state   *State
}

func (r *controllerRuntime) ListAllowedDAGs(ctx context.Context) ([]agent.AutomataAllowedDAG, error) {
	items, err := r.service.resolveAllowedDAGs(ctx, r.def)
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
	allowed, err := r.service.resolveAllowedDAGs(ctx, r.def)
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
	tagText := strings.Join([]string{
		fmt.Sprintf("automata=%s", strings.ToLower(r.def.Name)),
		fmt.Sprintf("automata_cycle=%s", r.state.CurrentCycleID),
	}, ",")
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

func (r *controllerRuntime) SetStage(ctx context.Context, stage, note string) error {
	if !slices.Contains(r.def.Stages, stage) {
		return fmt.Errorf("unknown stage %q", stage)
	}
	r.state.CurrentStage = stage
	r.state.StageChangedAt = r.service.clock()
	r.state.StageChangedBy = "agent"
	r.state.StageNote = note
	return r.service.saveState(ctx, r.def.Name, r.state)
}

func (r *controllerRuntime) RequestHumanInput(ctx context.Context, prompt agent.AutomataHumanPrompt) error {
	if strings.TrimSpace(prompt.Question) == "" {
		return fmt.Errorf("question is required")
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
	return r.service.saveState(ctx, r.def.Name, r.state)
}

func (r *controllerRuntime) Finish(ctx context.Context, summary string) error {
	r.state.State = StateFinished
	r.state.WaitingReason = WaitingReasonNone
	r.state.PendingPrompt = nil
	r.state.PendingResponse = nil
	r.state.CurrentRunRef = nil
	r.state.FinishedAt = r.service.clock()
	r.state.LastSummary = summary
	return r.service.saveState(ctx, r.def.Name, r.state)
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
