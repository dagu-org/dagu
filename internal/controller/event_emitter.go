// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"context"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

type controllerEventEmitter struct {
	service *Service
}

func (s *Service) eventEmitter() controllerEventEmitter {
	return controllerEventEmitter{service: s}
}

func (e controllerEventEmitter) sourceForContext(ctx context.Context) eventstore.Source {
	if source, ok := eventstore.SourceFromContext(ctx); ok {
		return source
	}
	return e.service.eventSource
}

func (e controllerEventEmitter) input(def *Definition, state *State) eventstore.ControllerEventInput {
	if def == nil || state == nil {
		return eventstore.ControllerEventInput{}
	}
	return eventstore.ControllerEventInput{
		Name:          def.Name,
		Kind:          string(normalizeControllerKind(def.Kind)),
		CycleID:       state.CurrentCycleID,
		SessionID:     state.SessionID,
		Status:        string(state.State),
		Summary:       state.LastSummary,
		Error:         state.LastError,
		OpenTaskCount: countTasksByState(state.Tasks, TaskStateOpen),
		DoneTaskCount: countTasksByState(state.Tasks, TaskStateDone),
	}
}

func (e controllerEventEmitter) emit(ctx context.Context, event *eventstore.Event) {
	if e.service == nil || e.service.eventService == nil || event == nil {
		return
	}
	if err := e.service.eventService.Emit(context.WithoutCancel(ctx), event); err != nil {
		e.service.logger.Warn("controller event emit failed",
			"controller", event.ControllerName,
			"type", event.Type,
			"error", err,
		)
	}
}

func (e controllerEventEmitter) needsInput(ctx context.Context, def *Definition, state *State) {
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

	e.emit(ctx, eventstore.NewControllerEvent(
		e.sourceForContext(ctx),
		eventstore.TypeControllerNeedsInput,
		eventstore.ControllerEventID(eventstore.TypeControllerNeedsInput, def.Name, state.PendingPrompt.ID),
		input,
		data,
	))
}

func (e controllerEventEmitter) finished(ctx context.Context, def *Definition, state *State) {
	if def == nil || state == nil {
		return
	}
	input := e.input(def, state)
	input.OccurredAt = state.FinishedAt
	e.emit(ctx, eventstore.NewControllerEvent(
		e.sourceForContext(ctx),
		eventstore.TypeControllerFinished,
		eventstore.ControllerEventID(eventstore.TypeControllerFinished, def.Name, state.CurrentCycleID),
		input,
		map[string]any{
			"summary":                  state.LastSummary,
			"current_task_description": input.CurrentTaskDescription,
			"open_task_count":          input.OpenTaskCount,
			"done_task_count":          input.DoneTaskCount,
		},
	))
}

func (e controllerEventEmitter) error(ctx context.Context, def *Definition, state *State, code string, occurredAt time.Time) {
	if def == nil || state == nil || strings.TrimSpace(state.LastError) == "" {
		return
	}
	input := e.input(def, state)
	input.OccurredAt = occurredAt
	e.emit(ctx, eventstore.NewControllerEvent(
		e.sourceForContext(ctx),
		eventstore.TypeControllerError,
		eventstore.ControllerEventID(eventstore.TypeControllerError, def.Name, state.CurrentCycleID, code, state.LastError),
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
