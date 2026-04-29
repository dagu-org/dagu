// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package autopilot

import (
	"context"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

type autopilotEventEmitter struct {
	service *Service
}

func (s *Service) eventEmitter() autopilotEventEmitter {
	return autopilotEventEmitter{service: s}
}

func (e autopilotEventEmitter) sourceForContext(ctx context.Context) eventstore.Source {
	if source, ok := eventstore.SourceFromContext(ctx); ok {
		return source
	}
	return e.service.eventSource
}

func (e autopilotEventEmitter) input(def *Definition, state *State) eventstore.AutopilotEventInput {
	if def == nil || state == nil {
		return eventstore.AutopilotEventInput{}
	}
	return eventstore.AutopilotEventInput{
		Name:          def.Name,
		Kind:          string(normalizeAutopilotKind(def.Kind)),
		CycleID:       state.CurrentCycleID,
		SessionID:     state.SessionID,
		Status:        string(state.State),
		Summary:       state.LastSummary,
		Error:         state.LastError,
		OpenTaskCount: countTasksByState(state.Tasks, TaskStateOpen),
		DoneTaskCount: countTasksByState(state.Tasks, TaskStateDone),
	}
}

func (e autopilotEventEmitter) emit(ctx context.Context, event *eventstore.Event) {
	if e.service == nil || e.service.eventService == nil || event == nil {
		return
	}
	if err := e.service.eventService.Emit(context.WithoutCancel(ctx), event); err != nil {
		e.service.logger.Warn("autopilot event emit failed",
			"autopilot", event.AutopilotName,
			"type", event.Type,
			"error", err,
		)
	}
}

func (e autopilotEventEmitter) needsInput(ctx context.Context, def *Definition, state *State) {
	if def == nil || state == nil || state.PendingPrompt == nil {
		return
	}
	input := e.input(def, state)
	input.OccurredAt = state.PendingPrompt.CreatedAt
	input.PromptID = state.PendingPrompt.ID
	input.PromptQuestion = state.PendingPrompt.Question

	data := map[string]any{
		"prompt_id":                state.PendingPrompt.ID,
		"prompt_question":          state.PendingPrompt.Question,
		"current_task_description": input.CurrentTaskDescription,
		"open_task_count":          input.OpenTaskCount,
		"done_task_count":          input.DoneTaskCount,
	}
	if len(state.PendingPrompt.Options) > 0 {
		data["options"] = append([]agent.UserPromptOption(nil), state.PendingPrompt.Options...)
	}
	if state.PendingPrompt.AllowFreeText {
		data["allow_free_text"] = true
	}
	if state.PendingPrompt.FreeTextPlaceholder != "" {
		data["free_text_placeholder"] = state.PendingPrompt.FreeTextPlaceholder
	}

	e.emit(ctx, eventstore.NewAutopilotEvent(
		e.sourceForContext(ctx),
		eventstore.TypeAutopilotNeedsInput,
		eventstore.AutopilotEventID(eventstore.TypeAutopilotNeedsInput, def.Name, state.PendingPrompt.ID),
		input,
		data,
	))
}

func (e autopilotEventEmitter) finished(ctx context.Context, def *Definition, state *State) {
	if def == nil || state == nil {
		return
	}
	input := e.input(def, state)
	input.OccurredAt = state.FinishedAt
	e.emit(ctx, eventstore.NewAutopilotEvent(
		e.sourceForContext(ctx),
		eventstore.TypeAutopilotFinished,
		eventstore.AutopilotEventID(eventstore.TypeAutopilotFinished, def.Name, state.CurrentCycleID),
		input,
		map[string]any{
			"summary":                  state.LastSummary,
			"current_task_description": input.CurrentTaskDescription,
			"open_task_count":          input.OpenTaskCount,
			"done_task_count":          input.DoneTaskCount,
		},
	))
}

func (e autopilotEventEmitter) error(ctx context.Context, def *Definition, state *State, code string, occurredAt time.Time) {
	if def == nil || state == nil || strings.TrimSpace(state.LastError) == "" {
		return
	}
	input := e.input(def, state)
	input.OccurredAt = occurredAt
	e.emit(ctx, eventstore.NewAutopilotEvent(
		e.sourceForContext(ctx),
		eventstore.TypeAutopilotError,
		eventstore.AutopilotEventID(eventstore.TypeAutopilotError, def.Name, state.CurrentCycleID, code, state.LastError),
		input,
		map[string]any{
			"error":                    state.LastError,
			"error_code":               code,
			"current_task_description": input.CurrentTaskDescription,
			"open_task_count":          input.OpenTaskCount,
			"done_task_count":          input.DoneTaskCount,
		},
	))
}
