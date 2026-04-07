// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/automata"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/audit"
)

const (
	auditActionAutomataMemoryUpdate = "memory_update"
	auditActionAutomataMemoryDelete = "memory_delete"
)

func (a *API) ListAutomata(ctx context.Context, _ api.ListAutomataRequestObject) (api.ListAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentAutomataControllerStatus(ctx)
	items, err := a.automataService.List(ctx)
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	resp := make([]api.AutomataSummary, 0, len(items))
	for _, item := range items {
		summary := toAPIAutomataSummary(item)
		summary.AutomataController = &controllerStatus
		resp = append(resp, summary)
	}
	return api.ListAutomata200JSONResponse{Automata: resp}, nil
}

func (a *API) GetAutomata(ctx context.Context, request api.GetAutomataRequestObject) (api.GetAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentAutomataControllerStatus(ctx)
	item, err := a.automataService.Detail(ctx, string(request.Name))
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	resp := toAPIAutomataDetail(item)
	resp.AutomataController = &controllerStatus
	return api.GetAutomata200JSONResponse(resp), nil
}

func (a *API) GetAutomataSpec(ctx context.Context, request api.GetAutomataSpecRequestObject) (api.GetAutomataSpecResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	spec, err := a.automataService.GetSpec(ctx, string(request.Name))
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	return api.GetAutomataSpec200JSONResponse{Spec: spec}, nil
}

func (a *API) GetAutomataMemory(ctx context.Context, request api.GetAutomataMemoryRequestObject) (api.GetAutomataMemoryResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireAutomataMemoryStore(); err != nil {
		return nil, err
	}
	item, err := a.automataService.GetMemory(ctx, string(request.Name))
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	return api.GetAutomataMemory200JSONResponse(toAPIAutomataMemory(item)), nil
}

func (a *API) UpdateAutomataMemory(ctx context.Context, request api.UpdateAutomataMemoryRequestObject) (api.UpdateAutomataMemoryResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireAutomataMemoryStore(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	item, err := a.automataService.SaveMemory(ctx, name, request.Body.Content)
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, auditActionAutomataMemoryUpdate, map[string]any{
		"name": name,
	})
	return api.UpdateAutomataMemory200JSONResponse(toAPIAutomataMemory(item)), nil
}

func (a *API) DeleteAutomataMemory(ctx context.Context, request api.DeleteAutomataMemoryRequestObject) (api.DeleteAutomataMemoryResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireAutomataMemoryStore(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.DeleteMemory(ctx, name); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, auditActionAutomataMemoryDelete, map[string]any{
		"name": name,
	})
	return api.DeleteAutomataMemory204Response{}, nil
}

func (a *API) PutAutomataSpec(ctx context.Context, request api.PutAutomataSpecRequestObject) (api.PutAutomataSpecResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.automataService.PutSpec(ctx, name, request.Body.Spec); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "spec_upsert", map[string]any{"name": name})
	return api.PutAutomataSpec204Response{}, nil
}

func (a *API) DeleteAutomata(ctx context.Context, request api.DeleteAutomataRequestObject) (api.DeleteAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.Delete(ctx, name); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "delete", map[string]any{"name": name})
	return api.DeleteAutomata204Response{}, nil
}

func (a *API) RenameAutomata(ctx context.Context, request api.RenameAutomataRequestObject) (api.RenameAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	body := automata.RenameRequest{
		NewName:     request.Body.NewName,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.automataService.Rename(ctx, name, body); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "rename", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.RenameAutomata204Response{}, nil
}

func (a *API) DuplicateAutomata(ctx context.Context, request api.DuplicateAutomataRequestObject) (api.DuplicateAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	body := automata.DuplicateRequest{NewName: request.Body.NewName}
	if err := a.automataService.Duplicate(ctx, name, body); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "duplicate", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.DuplicateAutomata204Response{}, nil
}

func (a *API) ResetAutomata(ctx context.Context, request api.ResetAutomataRequestObject) (api.ResetAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.ResetState(ctx, name); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "reset", map[string]any{"name": name})
	return api.ResetAutomata204Response{}, nil
}

func (a *API) StartAutomata(ctx context.Context, request api.StartAutomataRequestObject) (api.StartAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	body := automata.StartRequest{
		RequestedBy: a.currentUsername(ctx),
	}
	if request.Body != nil && request.Body.Instruction != nil {
		body.Instruction = *request.Body.Instruction
	}
	if err := a.automataService.RequestStart(ctx, name, body); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "start", map[string]any{
		"name":        name,
		"instruction": body.Instruction,
	})
	return api.StartAutomata204Response{}, nil
}

func (a *API) PauseAutomata(ctx context.Context, request api.PauseAutomataRequestObject) (api.PauseAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.Pause(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "pause", map[string]any{"name": name})
	return api.PauseAutomata204Response{}, nil
}

func (a *API) ResumeAutomata(ctx context.Context, request api.ResumeAutomataRequestObject) (api.ResumeAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.Resume(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "resume", map[string]any{"name": name})
	return api.ResumeAutomata204Response{}, nil
}

func (a *API) CreateAutomataTask(ctx context.Context, request api.CreateAutomataTaskRequestObject) (api.CreateAutomataTaskResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	task, err := a.automataService.CreateTask(ctx, name, automata.CreateTaskRequest{
		Description: request.Body.Description,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "task_create", map[string]any{
		"name": name,
		"id":   task.ID,
	})
	return api.CreateAutomataTask200JSONResponse(toAPIAutomataTask(task)), nil
}

func (a *API) UpdateAutomataTask(ctx context.Context, request api.UpdateAutomataTaskRequestObject) (api.UpdateAutomataTaskResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	task, err := a.automataService.UpdateTask(ctx, name, request.TaskId, automata.UpdateTaskRequest{
		Description: request.Body.Description,
		Done:        request.Body.Done,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "task_update", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.UpdateAutomataTask200JSONResponse(toAPIAutomataTask(task)), nil
}

func (a *API) DeleteAutomataTask(ctx context.Context, request api.DeleteAutomataTaskRequestObject) (api.DeleteAutomataTaskResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.automataService.DeleteTask(ctx, name, request.TaskId, a.currentUsername(ctx)); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "task_delete", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.DeleteAutomataTask204Response{}, nil
}

func (a *API) ReorderAutomataTasks(ctx context.Context, request api.ReorderAutomataTasksRequestObject) (api.ReorderAutomataTasksResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.automataService.ReorderTasks(ctx, name, automata.ReorderTasksRequest{
		TaskIDs:     append([]string(nil), request.Body.TaskIds...),
		RequestedBy: a.currentUsername(ctx),
	}); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "task_reorder", map[string]any{
		"name": name,
	})
	return api.ReorderAutomataTasks204Response{}, nil
}

func (a *API) MessageAutomata(ctx context.Context, request api.MessageAutomataRequestObject) (api.MessageAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	body := automata.OperatorMessageRequest{
		Message:     request.Body.Message,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.automataService.SubmitOperatorMessage(ctx, name, body); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "message", map[string]any{"name": name})
	return api.MessageAutomata204Response{}, nil
}

func (a *API) RespondAutomata(ctx context.Context, request api.RespondAutomataRequestObject) (api.RespondAutomataResponseObject, error) {
	if err := a.requireAutomataService(); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutomataController(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	body := automata.HumanResponseRequest{
		PromptID:          request.Body.PromptId,
		SelectedOptionIDs: append([]string(nil), valueOf(request.Body.SelectedOptionIds)...),
		FreeTextResponse:  valueOf(request.Body.FreeTextResponse),
	}
	if err := a.automataService.SubmitHumanResponse(ctx, name, body); err != nil {
		return nil, toAutomataAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutomata, "respond", map[string]any{
		"name":      name,
		"prompt_id": body.PromptID,
	})
	return api.RespondAutomata204Response{}, nil
}

func (a *API) requireAutomataService() error {
	if a.automataService == nil {
		return &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Automata service is not available",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	return nil
}

func (a *API) requireReadyAutomataController(ctx context.Context) error {
	status := a.currentAutomataControllerStatus(ctx)
	if status.State == api.AutomataControllerStatusStateReady {
		return nil
	}
	message := "No active scheduler with a ready Automata controller is available."
	if status.Message != nil && *status.Message != "" {
		message = *status.Message
	}
	return &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

func (a *API) requireAutomataMemoryStore() error {
	if a.agentMemoryStore == nil {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Automata memory management is not available",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

func (a *API) currentAutomataControllerStatus(ctx context.Context) api.AutomataControllerStatus {
	status := normalizeAutomataController(nil)
	if a.serviceRegistry == nil {
		status.State = exec.AutomataControllerStateUnavailable
		status.Message = "Service registry is not configured"
		return toAPIAutomataControllerStatus(status)
	}

	members, err := a.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameScheduler)
	if err != nil {
		status.State = exec.AutomataControllerStateUnavailable
		status.Message = "Failed to retrieve scheduler status"
		return toAPIAutomataControllerStatus(status)
	}

	var fallback exec.AutomataControllerInfo
	hasFallback := false
	for _, member := range members {
		if member.Status != exec.ServiceStatusActive {
			continue
		}
		normalized := normalizeAutomataController(member.AutomataController)
		if normalized.State == exec.AutomataControllerStateReady {
			return toAPIAutomataControllerStatus(normalized)
		}
		if !hasFallback {
			fallback = normalized
			hasFallback = true
		}
	}
	if hasFallback {
		return toAPIAutomataControllerStatus(fallback)
	}

	status.State = exec.AutomataControllerStateUnavailable
	status.Message = "No active scheduler with a ready Automata controller is available."
	return toAPIAutomataControllerStatus(status)
}

func normalizeAutomataController(info *exec.AutomataControllerInfo) exec.AutomataControllerInfo {
	if info == nil || info.State == "" {
		return exec.AutomataControllerInfo{
			State:   exec.AutomataControllerStateUnknown,
			Message: "Scheduler controller readiness is unknown",
		}
	}
	normalized := *info
	if normalized.Message == "" {
		switch normalized.State {
		case exec.AutomataControllerStateDisabled:
			normalized.Message = "Automata is disabled in agent settings"
		case exec.AutomataControllerStateUnavailable:
			normalized.Message = "Automata controller is unavailable"
		case exec.AutomataControllerStateUnknown:
			normalized.Message = "Scheduler controller readiness is unknown"
		}
	}
	return normalized
}

func toAPIAutomataControllerStatus(info exec.AutomataControllerInfo) api.AutomataControllerStatus {
	status := api.AutomataControllerStatus{
		State: api.AutomataControllerStatusState(info.State),
	}
	if info.Message != "" {
		status.Message = ptrOf(info.Message)
	}
	return status
}

func (a *API) currentUsername(ctx context.Context) string {
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		return ""
	}
	return user.Username
}

func toAutomataAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return err
	}
	switch {
	case errors.Is(err, exec.ErrDAGNotFound), errors.Is(err, os.ErrNotExist):
		return &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    err.Error(),
			HTTPStatus: http.StatusNotFound,
		}
	default:
		return &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
		}
	}
}

func toAPIAutomataSummary(item automata.Summary) api.AutomataSummary {
	return api.AutomataSummary{
		Busy: func() *bool {
			v := item.Busy
			return &v
		}(),
		CurrentRun:    toAPIAutomataRunSummary(item.CurrentRun),
		Description:   ptrOf(item.Description),
		Disabled:      ptrOf(item.Disabled),
		DisplayStatus: ptrOf(api.AutomataDisplayStatus(item.DisplayStatus)),
		DoneTaskCount: ptrOf(item.DoneTaskCount),
		Goal:          ptrOf(item.Goal),
		IconUrl:       ptrOf(item.IconURL),
		Instruction:   ptrOf(item.Instruction),
		Kind:          api.AutomataKind(item.Kind),
		LastUpdatedAt: ptrOf(item.LastUpdatedAt),
		Name:          item.Name,
		Nickname:      ptrOf(item.Nickname),
		NeedsInput: func() *bool {
			v := item.NeedsInput
			return &v
		}(),
		NextTaskDescription: ptrOf(item.NextTaskDescription),
		OpenTaskCount:       ptrOf(item.OpenTaskCount),
		State:               api.AutomataLifecycleState(item.State),
		Tags: func() *[]string {
			if len(item.Tags) == 0 {
				return nil
			}
			tags := append([]string(nil), item.Tags...)
			return &tags
		}(),
	}
}

func toAPIAutomataMemory(item *automata.Memory) api.AutomataMemoryResponse {
	if item == nil {
		return api.AutomataMemoryResponse{}
	}
	return api.AutomataMemoryResponse{
		Content: item.Content,
		Name:    item.Name,
		Path:    item.Path,
	}
}

func toAPIAutomataDetail(item *automata.Detail) api.AutomataDetailResponse {
	if item == nil {
		return api.AutomataDetailResponse{
			AllowedDags:   []api.AutomataAllowedDAGInfo{},
			TaskTemplates: &[]api.AutomataTaskTemplate{},
		}
	}
	resp := api.AutomataDetailResponse{
		AllowedDags: toAPIAutomataAllowedDAGInfos(item.AllowedDAGs),
		CurrentRun:  toAPIAutomataRunSummary(item.CurrentRun),
		Definition:  toAPIAutomataDefinition(item.Definition),
		State:       toAPIAutomataState(item.Definition, item.State),
	}
	taskTemplates := toAPIAutomataTaskTemplates(item.TaskTemplates)
	resp.TaskTemplates = &taskTemplates
	if len(item.RecentRuns) > 0 {
		runs := make([]api.AutomataRunSummary, 0, len(item.RecentRuns))
		for _, run := range item.RecentRuns {
			apiRun := toAPIAutomataRunSummary(&run)
			if apiRun != nil {
				runs = append(runs, *apiRun)
			}
		}
		resp.RecentRuns = &runs
	}
	if len(item.Messages) > 0 {
		msgs := toAPIAgentMessages(item.Messages)
		resp.Messages = &msgs
	}
	return resp
}

func toAPIAutomataDefinition(def *automata.Definition) api.AutomataDefinition {
	if def == nil {
		return api.AutomataDefinition{}
	}
	resp := api.AutomataDefinition{
		Description:         ptrOf(def.Description),
		Disabled:            ptrOf(def.Disabled),
		Goal:                ptrOf(def.Goal),
		IconUrl:             ptrOf(def.IconURL),
		Kind:                api.AutomataKind(def.Kind),
		Name:                def.Name,
		Nickname:            ptrOf(def.Nickname),
		AllowedDAGs:         toAPIAutomataAllowedDAGs(def.AllowedDAGs),
		StandingInstruction: ptrOf(def.StandingInstruction),
		Tags: func() *[]string {
			if len(def.Tags) == 0 {
				return nil
			}
			tags := append([]string(nil), def.Tags...)
			return &tags
		}(),
	}
	if len(def.Schedule) > 0 {
		schedule := make([]string, 0, len(def.Schedule))
		for _, item := range def.Schedule {
			if item.Expression != "" {
				schedule = append(schedule, item.Expression)
			}
		}
		resp.Schedule = &schedule
	}
	if agentConfig := toAPIAutomataAgentConfig(def.Agent); agentConfig != nil {
		resp.Agent = agentConfig
	}
	return resp
}

func toAPIAutomataAgentConfig(cfg automata.AgentConfig) *api.AutomataAgentConfig {
	resp := &api.AutomataAgentConfig{
		Model:    ptrOf(cfg.Model),
		SafeMode: new(cfg.SafeMode),
		Soul:     ptrOf(cfg.Soul),
	}
	if len(cfg.EnabledSkills) > 0 {
		skills := append([]string(nil), cfg.EnabledSkills...)
		resp.EnabledSkills = &skills
	}
	if resp.Model == nil && resp.SafeMode == nil && resp.Soul == nil && resp.EnabledSkills == nil {
		return nil
	}
	return resp
}

func toAPIAutomataAllowedDAGs(allowed automata.AllowedDAGs) *api.AutomataAllowedDAGs {
	resp := &api.AutomataAllowedDAGs{}
	if len(allowed.Names) > 0 {
		names := append([]string(nil), allowed.Names...)
		resp.Names = &names
	}
	if len(allowed.Tags) > 0 {
		tags := append([]string(nil), allowed.Tags...)
		resp.Tags = &tags
	}
	if resp.Names == nil && resp.Tags == nil {
		return nil
	}
	return resp
}

func toAPIAutomataAllowedDAGInfos(items []automata.AllowedDAGInfo) []api.AutomataAllowedDAGInfo {
	if len(items) == 0 {
		return []api.AutomataAllowedDAGInfo{}
	}
	resp := make([]api.AutomataAllowedDAGInfo, 0, len(items))
	for _, item := range items {
		apiItem := api.AutomataAllowedDAGInfo{
			Description: ptrOf(item.Description),
			Name:        item.Name,
		}
		if len(item.Tags) > 0 {
			tags := append([]string(nil), item.Tags...)
			apiItem.Tags = &tags
		}
		resp = append(resp, apiItem)
	}
	return resp
}

func toAPIAutomataState(def *automata.Definition, state *automata.State) api.AutomataState {
	if state == nil {
		return api.AutomataState{}
	}
	view := automata.DeriveView(def, state)
	resp := api.AutomataState{
		ActivatedAt: ptrOf(state.ActivatedAt),
		ActivatedBy: ptrOf(state.ActivatedBy),
		Busy: func() *bool {
			v := view.Busy
			return &v
		}(),
		CurrentCycleId:       ptrOf(state.CurrentCycleID),
		CurrentRunRef:        toAPIAutomataRunRef(state.CurrentRunRef),
		DisplayStatus:        ptrOf(api.AutomataDisplayStatus(view.DisplayStatus)),
		FinishedAt:           ptrOf(state.FinishedAt),
		Instruction:          ptrOf(state.Instruction),
		InstructionUpdatedAt: ptrOf(state.InstructionUpdatedAt),
		InstructionUpdatedBy: ptrOf(state.InstructionUpdatedBy),
		LastError:            ptrOf(state.LastError),
		LastRunRef:           toAPIAutomataRunRef(state.LastRunRef),
		LastScheduleMinute:   ptrOf(state.LastScheduleMinute),
		LastSummary:          ptrOf(state.LastSummary),
		LastTriggeredAt:      ptrOf(state.LastTriggeredAt),
		LastUpdatedAt:        ptrOf(state.LastUpdatedAt),
		NeedsInput: func() *bool {
			v := view.NeedsInput
			return &v
		}(),
		PausedAt:         ptrOf(state.PausedAt),
		PausedBy:         ptrOf(state.PausedBy),
		PendingPrompt:    toAPIAutomataPrompt(state.PendingPrompt),
		PendingResponse:  toAPIAutomataPromptResponse(state.PendingResponse),
		SessionId:        ptrOf(state.SessionID),
		StartRequestedAt: ptrOf(state.StartRequestedAt),
		State:            api.AutomataLifecycleState(state.State),
		WaitingReason:    toAPIAutomataWaitingReason(state.WaitingReason),
	}
	if len(state.Tasks) > 0 {
		tasks := toAPIAutomataTasks(state.Tasks)
		resp.Tasks = &tasks
	}
	if len(state.PendingTurnMessages) > 0 {
		messages := toAPIAutomataPendingTurnMessages(state.PendingTurnMessages)
		resp.PendingTurnMessages = &messages
	}
	return resp
}

func toAPIAutomataWaitingReason(reason automata.WaitingReason) *api.AutomataWaitingReason {
	if reason == "" {
		return nil
	}
	apiReason := api.AutomataWaitingReason(reason)
	return &apiReason
}

func toAPIAutomataPrompt(prompt *automata.Prompt) *api.AutomataPrompt {
	if prompt == nil {
		return nil
	}
	resp := &api.AutomataPrompt{
		AllowFreeText:       new(prompt.AllowFreeText),
		CreatedAt:           prompt.CreatedAt,
		FreeTextPlaceholder: ptrOf(prompt.FreeTextPlaceholder),
		Id:                  prompt.ID,
		Question:            prompt.Question,
	}
	if len(prompt.Options) > 0 {
		options := toAPIAgentUserPromptOptions(prompt.Options)
		resp.Options = &options
	}
	return resp
}

func toAPIAutomataPromptResponse(response *automata.PromptResponse) *api.AutomataPromptResponse {
	if response == nil {
		return nil
	}
	resp := &api.AutomataPromptResponse{
		FreeTextResponse: ptrOf(response.FreeTextResponse),
		PromptId:         response.PromptID,
		RespondedAt:      response.RespondedAt,
	}
	if len(response.SelectedOptionIDs) > 0 {
		selected := append([]string(nil), response.SelectedOptionIDs...)
		resp.SelectedOptionIds = &selected
	}
	return resp
}

func toAPIAutomataPendingTurnMessages(
	messages []automata.PendingTurnMessage,
) []api.AutomataPendingTurnMessage {
	resp := make([]api.AutomataPendingTurnMessage, 0, len(messages))
	for _, message := range messages {
		resp = append(resp, api.AutomataPendingTurnMessage{
			CreatedAt: message.CreatedAt,
			Id:        message.ID,
			Kind:      message.Kind,
			Message:   message.Message,
		})
	}
	return resp
}

func toAPIAutomataTask(task *automata.Task) api.AutomataTask {
	if task == nil {
		return api.AutomataTask{}
	}
	return api.AutomataTask{
		CreatedAt:   ptrOf(task.CreatedAt),
		CreatedBy:   ptrOf(task.CreatedBy),
		Description: task.Description,
		DoneAt:      ptrOf(task.DoneAt),
		DoneBy:      ptrOf(task.DoneBy),
		Id:          task.ID,
		State:       api.AutomataTaskState(task.State),
		UpdatedAt:   ptrOf(task.UpdatedAt),
		UpdatedBy:   ptrOf(task.UpdatedBy),
	}
}

func toAPIAutomataTasks(tasks []automata.Task) []api.AutomataTask {
	resp := make([]api.AutomataTask, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]
		resp = append(resp, toAPIAutomataTask(&task))
	}
	return resp
}

func toAPIAutomataTaskTemplates(tasks []automata.TaskTemplate) []api.AutomataTaskTemplate {
	resp := make([]api.AutomataTaskTemplate, 0, len(tasks))
	for _, task := range tasks {
		resp = append(resp, api.AutomataTaskTemplate{
			CreatedAt:   ptrOf(task.CreatedAt),
			CreatedBy:   ptrOf(task.CreatedBy),
			Description: task.Description,
			Id:          task.ID,
			UpdatedAt:   ptrOf(task.UpdatedAt),
			UpdatedBy:   ptrOf(task.UpdatedBy),
		})
	}
	return resp
}

func toAPIAutomataRunRef(ref *exec.DAGRunRef) *api.AutomataRunRef {
	if ref == nil {
		return nil
	}
	return &api.AutomataRunRef{
		Id:   ref.ID,
		Name: ref.Name,
	}
}

func toAPIAutomataRunSummary(run *automata.RunSummary) *api.AutomataRunSummary {
	if run == nil {
		return nil
	}
	resp := &api.AutomataRunSummary{
		CreatedAt:  ptrOf(run.CreatedAt),
		DagRunId:   run.DAGRunID,
		Error:      ptrOf(run.Error),
		FinishedAt: ptrOf(run.FinishedAt),
		Name:       run.Name,
		StartedAt:  ptrOf(run.StartedAt),
		Status:     api.StatusLabel(run.Status),
		TriggerType: func() *api.TriggerType {
			if run.TriggerType == "" {
				return nil
			}
			triggerType := api.TriggerType(run.TriggerType)
			return &triggerType
		}(),
	}
	return resp
}

func toAPIAgentMessages(messages []agent.Message) []api.AgentMessage {
	resp := make([]api.AgentMessage, 0, len(messages))
	for _, message := range messages {
		resp = append(resp, toAPIAgentMessage(message))
	}
	return resp
}

func toAPIAgentMessage(message agent.Message) api.AgentMessage {
	resp := api.AgentMessage{
		Content:    ptrOf(message.Content),
		Cost:       message.Cost,
		CreatedAt:  message.CreatedAt,
		Id:         message.ID,
		SequenceId: message.SequenceID,
		SessionId:  message.SessionID,
		Type:       api.AgentMessageType(message.Type),
	}
	if len(message.DelegateIDs) > 0 {
		delegateIDs := append([]string(nil), message.DelegateIDs...)
		resp.DelegateIds = &delegateIDs
	}
	if len(message.ToolCalls) > 0 {
		toolCalls := make([]api.AgentToolCall, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			toolCalls = append(toolCalls, api.AgentToolCall{
				Function: api.AgentToolCallFunction{
					Arguments: call.Function.Arguments,
					Name:      call.Function.Name,
				},
				Id:   call.ID,
				Type: call.Type,
			})
		}
		resp.ToolCalls = &toolCalls
	}
	if len(message.ToolResults) > 0 {
		toolResults := make([]api.AgentToolResult, 0, len(message.ToolResults))
		for _, result := range message.ToolResults {
			toolResults = append(toolResults, api.AgentToolResult{
				Content:    result.Content,
				IsError:    new(result.IsError),
				ToolCallId: result.ToolCallID,
			})
		}
		resp.ToolResults = &toolResults
	}
	if message.Usage != nil {
		resp.Usage = &api.AgentTokenUsage{
			CompletionTokens: new(message.Usage.CompletionTokens),
			PromptTokens:     new(message.Usage.PromptTokens),
			TotalTokens:      new(message.Usage.TotalTokens),
		}
	}
	if message.UIAction != nil {
		resp.UiAction = &api.AgentUIAction{
			Path: ptrOf(message.UIAction.Path),
			Type: string(message.UIAction.Type),
		}
	}
	if message.UserPrompt != nil {
		resp.UserPrompt = toAPIAgentUserPrompt(message.UserPrompt)
	}
	return resp
}

func toAPIAgentUserPrompt(prompt *agent.UserPrompt) *api.AgentUserPrompt {
	if prompt == nil {
		return nil
	}
	resp := &api.AgentUserPrompt{
		AllowFreeText:       prompt.AllowFreeText,
		Command:             ptrOf(prompt.Command),
		FreeTextPlaceholder: ptrOf(prompt.FreeTextPlaceholder),
		MultiSelect:         prompt.MultiSelect,
		PromptId:            prompt.PromptID,
		Question:            prompt.Question,
		WorkingDir:          ptrOf(prompt.WorkingDir),
	}
	if prompt.PromptType != "" {
		promptType := api.AgentUserPromptPromptType(prompt.PromptType)
		resp.PromptType = &promptType
	}
	if len(prompt.Options) > 0 {
		options := toAPIAgentUserPromptOptions(prompt.Options)
		resp.Options = &options
	}
	return resp
}

func toAPIAgentUserPromptOptions(options []agent.UserPromptOption) []api.AgentUserPromptOption {
	resp := make([]api.AgentUserPromptOption, 0, len(options))
	for _, option := range options {
		resp = append(resp, api.AgentUserPromptOption{
			Description: ptrOf(option.Description),
			Id:          option.ID,
			Label:       option.Label,
		})
	}
	return resp
}
