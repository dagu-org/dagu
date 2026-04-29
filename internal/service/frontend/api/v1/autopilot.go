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
	"github.com/dagucloud/dagu/internal/autopilot"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/audit"
	"gopkg.in/yaml.v3"
)

const (
	auditActionAutopilotDocumentUpdate = "document_update"
	auditActionAutopilotDocumentDelete = "document_delete"
)

func (a *API) ListAutopilot(ctx context.Context, _ api.ListAutopilotRequestObject) (api.ListAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentAutopilotControllerStatus(ctx)
	items, err := a.autopilotService.List(ctx)
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	resp := make([]api.AutopilotSummary, 0, len(items))
	for _, item := range items {
		if !a.canAccessWorkspace(ctx, autopilotWorkspaceNameFromTags(item.Tags)) {
			continue
		}
		summary := toAPIAutopilotSummary(item)
		summary.AutopilotController = &controllerStatus
		resp = append(resp, summary)
	}
	return api.ListAutopilot200JSONResponse{Autopilot: resp}, nil
}

func (a *API) GetAutopilot(ctx context.Context, request api.GetAutopilotRequestObject) (api.GetAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentAutopilotControllerStatus(ctx)
	item, err := a.autopilotService.Detail(ctx, string(request.Name))
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	if err := a.requireWorkspaceVisible(ctx, autopilotWorkspaceNameFromDetail(item)); err != nil {
		return nil, err
	}
	resp := toAPIAutopilotDetail(item)
	resp.AutopilotController = &controllerStatus
	return api.GetAutopilot200JSONResponse(resp), nil
}

func (a *API) GetAutopilotSpec(ctx context.Context, request api.GetAutopilotSpecRequestObject) (api.GetAutopilotSpecResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotVisible(ctx, string(request.Name)); err != nil {
		return nil, err
	}
	spec, err := a.autopilotService.GetSpec(ctx, string(request.Name))
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	return api.GetAutopilotSpec200JSONResponse{Spec: spec}, nil
}

func (a *API) GetAutopilotDocument(ctx context.Context, request api.GetAutopilotDocumentRequestObject) (api.GetAutopilotDocumentResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotDocumentStore(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotVisible(ctx, string(request.Name)); err != nil {
		return nil, err
	}
	item, err := a.autopilotService.GetDocument(ctx, string(request.Name), string(request.Document))
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	return api.GetAutopilotDocument200JSONResponse(toAPIAutopilotDocument(item)), nil
}

func (a *API) UpdateAutopilotDocument(ctx context.Context, request api.UpdateAutopilotDocumentRequestObject) (api.UpdateAutopilotDocumentResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotDocumentStore(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireAutopilotDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	document := string(request.Document)
	item, err := a.autopilotService.SaveDocument(ctx, name, document, request.Body.Content)
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, auditActionAutopilotDocumentUpdate, map[string]any{
		"name":     name,
		"document": document,
	})
	return api.UpdateAutopilotDocument200JSONResponse(toAPIAutopilotDocument(item)), nil
}

func (a *API) DeleteAutopilotDocument(ctx context.Context, request api.DeleteAutopilotDocumentRequestObject) (api.DeleteAutopilotDocumentResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotDocumentStore(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	document := string(request.Document)
	if err := a.autopilotService.DeleteDocument(ctx, name, document); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, auditActionAutopilotDocumentDelete, map[string]any{
		"name":     name,
		"document": document,
	})
	return api.DeleteAutopilotDocument204Response{}, nil
}

func (a *API) PutAutopilotSpec(ctx context.Context, request api.PutAutopilotSpecRequestObject) (api.PutAutopilotSpecResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireAutopilotSpecWrite(ctx, name, request.Body.Spec); err != nil {
		return nil, err
	}
	if err := a.autopilotService.PutSpec(ctx, name, request.Body.Spec); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "spec_upsert", map[string]any{"name": name})
	return api.PutAutopilotSpec204Response{}, nil
}

func (a *API) DeleteAutopilot(ctx context.Context, request api.DeleteAutopilotRequestObject) (api.DeleteAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	if err := a.autopilotService.Delete(ctx, name); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "delete", map[string]any{"name": name})
	return api.DeleteAutopilot204Response{}, nil
}

func (a *API) RenameAutopilot(ctx context.Context, request api.RenameAutopilotRequestObject) (api.RenameAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireAutopilotDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	body := autopilot.RenameRequest{
		NewName:     request.Body.NewName,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.autopilotService.Rename(ctx, name, body); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "rename", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.RenameAutopilot204Response{}, nil
}

func (a *API) DuplicateAutopilot(ctx context.Context, request api.DuplicateAutopilotRequestObject) (api.DuplicateAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireAutopilotDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	body := autopilot.DuplicateRequest{NewName: request.Body.NewName}
	if err := a.autopilotService.Duplicate(ctx, name, body); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "duplicate", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.DuplicateAutopilot204Response{}, nil
}

func (a *API) ResetAutopilot(ctx context.Context, request api.ResetAutopilotRequestObject) (api.ResetAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	if err := a.autopilotService.ResetState(ctx, name); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "reset", map[string]any{"name": name})
	return api.ResetAutopilot204Response{}, nil
}

func (a *API) StartAutopilot(ctx context.Context, request api.StartAutopilotRequestObject) (api.StartAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	body := autopilot.StartRequest{
		RequestedBy: a.currentUsername(ctx),
	}
	if request.Body != nil && request.Body.Instruction != nil {
		body.Instruction = *request.Body.Instruction
	}
	if err := a.autopilotService.RequestStart(ctx, name, body); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "start", map[string]any{
		"name":        name,
		"instruction": body.Instruction,
	})
	return api.StartAutopilot204Response{}, nil
}

func (a *API) PauseAutopilot(ctx context.Context, request api.PauseAutopilotRequestObject) (api.PauseAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	if err := a.autopilotService.Pause(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "pause", map[string]any{"name": name})
	return api.PauseAutopilot204Response{}, nil
}

func (a *API) ResumeAutopilot(ctx context.Context, request api.ResumeAutopilotRequestObject) (api.ResumeAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	if err := a.autopilotService.Resume(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "resume", map[string]any{"name": name})
	return api.ResumeAutopilot204Response{}, nil
}

func (a *API) CreateAutopilotTask(ctx context.Context, request api.CreateAutopilotTaskRequestObject) (api.CreateAutopilotTaskResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	task, err := a.autopilotService.CreateTask(ctx, name, autopilot.CreateTaskRequest{
		Description: request.Body.Description,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "task_create", map[string]any{
		"name": name,
		"id":   task.ID,
	})
	return api.CreateAutopilotTask200JSONResponse(toAPIAutopilotTask(task)), nil
}

func (a *API) UpdateAutopilotTask(ctx context.Context, request api.UpdateAutopilotTaskRequestObject) (api.UpdateAutopilotTaskResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	task, err := a.autopilotService.UpdateTask(ctx, name, request.TaskId, autopilot.UpdateTaskRequest{
		Description: request.Body.Description,
		Done:        request.Body.Done,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "task_update", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.UpdateAutopilotTask200JSONResponse(toAPIAutopilotTask(task)), nil
}

func (a *API) DeleteAutopilotTask(ctx context.Context, request api.DeleteAutopilotTaskRequestObject) (api.DeleteAutopilotTaskResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.autopilotService.DeleteTask(ctx, name, request.TaskId, a.currentUsername(ctx)); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "task_delete", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.DeleteAutopilotTask204Response{}, nil
}

func (a *API) ReorderAutopilotTasks(ctx context.Context, request api.ReorderAutopilotTasksRequestObject) (api.ReorderAutopilotTasksResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	if err := a.autopilotService.ReorderTasks(ctx, name, autopilot.ReorderTasksRequest{
		TaskIDs:     append([]string(nil), request.Body.TaskIds...),
		RequestedBy: a.currentUsername(ctx),
	}); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "task_reorder", map[string]any{
		"name": name,
	})
	return api.ReorderAutopilotTasks204Response{}, nil
}

func (a *API) MessageAutopilot(ctx context.Context, request api.MessageAutopilotRequestObject) (api.MessageAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	body := autopilot.OperatorMessageRequest{
		Message:     request.Body.Message,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.autopilotService.SubmitOperatorMessage(ctx, name, body); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "message", map[string]any{"name": name})
	return api.MessageAutopilot204Response{}, nil
}

func (a *API) RespondAutopilot(ctx context.Context, request api.RespondAutopilotRequestObject) (api.RespondAutopilotResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireAutopilotExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyAutopilotController(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	body := autopilot.HumanResponseRequest{
		PromptID:          request.Body.PromptId,
		SelectedOptionIDs: append([]string(nil), valueOf(request.Body.SelectedOptionIds)...),
		FreeTextResponse:  valueOf(request.Body.FreeTextResponse),
	}
	if err := a.autopilotService.SubmitHumanResponse(ctx, name, body); err != nil {
		return nil, toAutopilotAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryAutopilot, "respond", map[string]any{
		"name":      name,
		"prompt_id": body.PromptID,
	})
	return api.RespondAutopilot204Response{}, nil
}

func autopilotWorkspaceNameFromTags(tags []string) string {
	workspaceName, state := exec.WorkspaceLabelFromLabels(core.NewLabels(tags))
	switch state {
	case exec.WorkspaceLabelValid:
		return workspaceName
	case exec.WorkspaceLabelInvalid:
		return invalidWorkspaceLabelName
	case exec.WorkspaceLabelMissing:
		return ""
	default:
		return ""
	}
}

func autopilotWorkspaceNameFromDefinition(def *autopilot.Definition) string {
	if def == nil {
		return ""
	}
	return autopilotWorkspaceNameFromTags(def.Tags)
}

func autopilotWorkspaceNameFromDetail(detail *autopilot.Detail) string {
	if detail == nil {
		return ""
	}
	return autopilotWorkspaceNameFromDefinition(detail.Definition)
}

func autopilotWorkspaceNameFromSpec(spec string) (string, error) {
	var def autopilot.Definition
	if err := yaml.Unmarshal([]byte(spec), &def); err != nil {
		return "", err
	}
	return autopilotWorkspaceNameFromDefinition(&def), nil
}

func (a *API) autopilotWorkspaceName(ctx context.Context, name string) (string, error) {
	spec, err := a.autopilotService.GetSpec(ctx, name)
	if err != nil {
		return "", err
	}
	workspaceName, err := autopilotWorkspaceNameFromSpec(spec)
	if err != nil {
		return "", nil
	}
	return workspaceName, nil
}

func (a *API) requireAutopilotVisible(ctx context.Context, name string) error {
	workspaceName, err := a.autopilotWorkspaceName(ctx, name)
	if err != nil {
		return toAutopilotAPIError(err)
	}
	return a.requireWorkspaceVisible(ctx, workspaceName)
}

func (a *API) requireAutopilotDAGWrite(ctx context.Context, name string) error {
	workspaceName, err := a.autopilotWorkspaceName(ctx, name)
	if err != nil {
		return toAutopilotAPIError(err)
	}
	return a.requireDAGWriteForWorkspace(ctx, workspaceName)
}

func (a *API) requireAutopilotExecute(ctx context.Context, name string) error {
	workspaceName, err := a.autopilotWorkspaceName(ctx, name)
	if err != nil {
		return toAutopilotAPIError(err)
	}
	return a.requireExecuteForWorkspace(ctx, workspaceName)
}

func (a *API) requireAutopilotSpecWrite(ctx context.Context, name, spec string) error {
	currentWorkspaceName, err := a.autopilotWorkspaceName(ctx, name)
	if err == nil {
		if err := a.requireDAGWriteForWorkspace(ctx, currentWorkspaceName); err != nil {
			return err
		}
	} else if !errors.Is(err, exec.ErrDAGNotFound) && !errors.Is(err, os.ErrNotExist) {
		return toAutopilotAPIError(err)
	}

	nextWorkspaceName, err := autopilotWorkspaceNameFromSpec(spec)
	if err != nil {
		return a.requireDAGWrite(ctx)
	}
	return a.requireDAGWriteForWorkspace(ctx, nextWorkspaceName)
}

func (a *API) requireAutopilotService() error {
	if a.autopilotService == nil {
		return &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Autopilot service is not available",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	return nil
}

func (a *API) requireReadyAutopilotController(ctx context.Context) error {
	status := a.currentAutopilotControllerStatus(ctx)
	if status.State == api.AutopilotControllerStatusStateReady {
		return nil
	}
	message := "No active scheduler with a ready Autopilot controller is available."
	if status.Message != nil && *status.Message != "" {
		message = *status.Message
	}
	return &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

func (a *API) requireAutopilotDocumentStore() error {
	if _, ok := a.agentMemoryStore.(agent.AutopilotDocumentStore); !ok {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Autopilot document management is not available",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

func (a *API) currentAutopilotControllerStatus(ctx context.Context) api.AutopilotControllerStatus {
	status := normalizeAutopilotController(nil)
	if a.serviceRegistry == nil {
		status.State = exec.AutopilotControllerStateUnavailable
		status.Message = "Service registry is not configured"
		return toAPIAutopilotControllerStatus(status)
	}

	members, err := a.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameScheduler)
	if err != nil {
		status.State = exec.AutopilotControllerStateUnavailable
		status.Message = "Failed to retrieve scheduler status"
		return toAPIAutopilotControllerStatus(status)
	}

	var fallback exec.AutopilotControllerInfo
	hasFallback := false
	for _, member := range members {
		if member.Status != exec.ServiceStatusActive {
			continue
		}
		normalized := normalizeAutopilotController(member.AutopilotController)
		if normalized.State == exec.AutopilotControllerStateReady {
			return toAPIAutopilotControllerStatus(normalized)
		}
		if !hasFallback {
			fallback = normalized
			hasFallback = true
		}
	}
	if hasFallback {
		return toAPIAutopilotControllerStatus(fallback)
	}

	status.State = exec.AutopilotControllerStateUnavailable
	status.Message = "No active scheduler with a ready Autopilot controller is available."
	return toAPIAutopilotControllerStatus(status)
}

func normalizeAutopilotController(info *exec.AutopilotControllerInfo) exec.AutopilotControllerInfo {
	if info == nil || info.State == "" {
		return exec.AutopilotControllerInfo{
			State:   exec.AutopilotControllerStateUnknown,
			Message: "Scheduler controller readiness is unknown",
		}
	}
	normalized := *info
	if normalized.Message == "" {
		switch normalized.State {
		case exec.AutopilotControllerStateReady:
		case exec.AutopilotControllerStateDisabled:
			normalized.Message = "Autopilot is disabled in agent settings"
		case exec.AutopilotControllerStateUnavailable:
			normalized.Message = "Autopilot controller is unavailable"
		case exec.AutopilotControllerStateUnknown:
			normalized.Message = "Scheduler controller readiness is unknown"
		}
	}
	return normalized
}

func toAPIAutopilotControllerStatus(info exec.AutopilotControllerInfo) api.AutopilotControllerStatus {
	status := api.AutopilotControllerStatus{
		State: api.AutopilotControllerStatusState(info.State),
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

func toAutopilotAPIError(err error) error {
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

func toAPIAutopilotSummary(item autopilot.Summary) api.AutopilotSummary {
	return api.AutopilotSummary{
		Busy: func() *bool {
			v := item.Busy
			return &v
		}(),
		CurrentRun:    toAPIAutopilotRunSummary(item.CurrentRun),
		ClonedFrom:    ptrOf(item.ClonedFrom),
		Description:   ptrOf(item.Description),
		Disabled:      ptrOf(item.Disabled),
		DisplayStatus: ptrOf(api.AutopilotDisplayStatus(item.DisplayStatus)),
		DoneTaskCount: ptrOf(item.DoneTaskCount),
		Goal:          ptrOf(item.Goal),
		IconUrl:       ptrOf(item.IconURL),
		Instruction:   ptrOf(item.Instruction),
		Kind:          api.AutopilotKind(item.Kind),
		LastUpdatedAt: ptrOf(item.LastUpdatedAt),
		Name:          item.Name,
		Nickname:      ptrOf(item.Nickname),
		NeedsInput: func() *bool {
			v := item.NeedsInput
			return &v
		}(),
		NextTaskDescription: ptrOf(item.NextTaskDescription),
		OpenTaskCount:       ptrOf(item.OpenTaskCount),
		ResetOnFinish:       ptrOf(item.ResetOnFinish),
		State:               api.AutopilotLifecycleState(item.State),
		Tags: func() *[]string {
			if len(item.Tags) == 0 {
				return nil
			}
			tags := append([]string(nil), item.Tags...)
			return &tags
		}(),
	}
}

func toAPIAutopilotDocument(item *autopilot.Document) api.AutopilotDocumentResponse {
	if item == nil {
		return api.AutopilotDocumentResponse{}
	}
	return api.AutopilotDocumentResponse{
		Content:  item.Content,
		Document: api.AutopilotDocument(item.Document),
		Name:     item.Name,
		Path:     item.Path,
	}
}

func toAPIAutopilotDetail(item *autopilot.Detail) api.AutopilotDetailResponse {
	if item == nil {
		return api.AutopilotDetailResponse{
			AllowedDags:   []api.AutopilotAllowedDAGInfo{},
			TaskTemplates: &[]api.AutopilotTaskTemplate{},
		}
	}
	resp := api.AutopilotDetailResponse{
		AllowedDags: toAPIAutopilotAllowedDAGInfos(item.AllowedDAGs),
		CurrentRun:  toAPIAutopilotRunSummary(item.CurrentRun),
		Definition:  toAPIAutopilotDefinition(item.Definition),
		State:       toAPIAutopilotState(item.Definition, item.State),
	}
	taskTemplates := toAPIAutopilotTaskTemplates(item.TaskTemplates)
	resp.TaskTemplates = &taskTemplates
	if len(item.RecentRuns) > 0 {
		runs := make([]api.AutopilotRunSummary, 0, len(item.RecentRuns))
		for _, run := range item.RecentRuns {
			apiRun := toAPIAutopilotRunSummary(&run)
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

func toAPIAutopilotDefinition(def *autopilot.Definition) api.AutopilotDefinition {
	if def == nil {
		return api.AutopilotDefinition{}
	}
	resp := api.AutopilotDefinition{
		Description:         ptrOf(def.Description),
		Disabled:            ptrOf(def.Disabled),
		Goal:                ptrOf(def.Goal),
		IconUrl:             ptrOf(def.IconURL),
		Kind:                api.AutopilotKind(def.Kind),
		Name:                def.Name,
		Nickname:            ptrOf(def.Nickname),
		ClonedFrom:          ptrOf(def.ClonedFrom),
		AllowedDAGs:         toAPIAutopilotAllowedDAGs(def.AllowedDAGs),
		ResetOnFinish:       ptrOf(def.ResetOnFinish),
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
	if agentConfig := toAPIAutopilotAgentConfig(def.Agent); agentConfig != nil {
		resp.Agent = agentConfig
	}
	return resp
}

func toAPIAutopilotAgentConfig(cfg autopilot.AgentConfig) *api.AutopilotAgentConfig {
	resp := &api.AutopilotAgentConfig{
		Model:    ptrOf(cfg.Model),
		SafeMode: new(cfg.SafeMode),
		Soul:     ptrOf(cfg.Soul),
	}
	if resp.Model == nil && resp.SafeMode == nil && resp.Soul == nil {
		return nil
	}
	return resp
}

func toAPIAutopilotAllowedDAGs(allowed autopilot.AllowedDAGs) *api.AutopilotAllowedDAGs {
	resp := &api.AutopilotAllowedDAGs{}
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

func toAPIAutopilotAllowedDAGInfos(items []autopilot.AllowedDAGInfo) []api.AutopilotAllowedDAGInfo {
	if len(items) == 0 {
		return []api.AutopilotAllowedDAGInfo{}
	}
	resp := make([]api.AutopilotAllowedDAGInfo, 0, len(items))
	for _, item := range items {
		apiItem := api.AutopilotAllowedDAGInfo{
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

func toAPIAutopilotState(def *autopilot.Definition, state *autopilot.State) api.AutopilotState {
	if state == nil {
		return api.AutopilotState{}
	}
	view := autopilot.DeriveView(def, state)
	resp := api.AutopilotState{
		ActivatedAt: ptrOf(state.ActivatedAt),
		ActivatedBy: ptrOf(state.ActivatedBy),
		Busy: func() *bool {
			v := view.Busy
			return &v
		}(),
		CurrentCycleId:       ptrOf(state.CurrentCycleID),
		CurrentRunRef:        toAPIAutopilotRunRef(state.CurrentRunRef),
		DisplayStatus:        ptrOf(api.AutopilotDisplayStatus(view.DisplayStatus)),
		FinishedAt:           ptrOf(state.FinishedAt),
		Instruction:          ptrOf(state.Instruction),
		InstructionUpdatedAt: ptrOf(state.InstructionUpdatedAt),
		InstructionUpdatedBy: ptrOf(state.InstructionUpdatedBy),
		LastError:            ptrOf(state.LastError),
		LastRunRef:           toAPIAutopilotRunRef(state.LastRunRef),
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
		PendingPrompt:    toAPIAutopilotPrompt(state.PendingPrompt),
		PendingResponse:  toAPIAutopilotPromptResponse(state.PendingResponse),
		SessionId:        ptrOf(state.SessionID),
		StartRequestedAt: ptrOf(state.StartRequestedAt),
		State:            api.AutopilotLifecycleState(state.State),
		WaitingReason:    toAPIAutopilotWaitingReason(state.WaitingReason),
	}
	if len(state.Tasks) > 0 {
		tasks := toAPIAutopilotTasks(state.Tasks)
		resp.Tasks = &tasks
	}
	if len(state.PendingTurnMessages) > 0 {
		messages := toAPIAutopilotPendingTurnMessages(state.PendingTurnMessages)
		resp.PendingTurnMessages = &messages
	}
	return resp
}

func toAPIAutopilotWaitingReason(reason autopilot.WaitingReason) *api.AutopilotWaitingReason {
	if reason == "" {
		return nil
	}
	apiReason := api.AutopilotWaitingReason(reason)
	return &apiReason
}

func toAPIAutopilotPrompt(prompt *autopilot.Prompt) *api.AutopilotPrompt {
	if prompt == nil {
		return nil
	}
	resp := &api.AutopilotPrompt{
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

func toAPIAutopilotPromptResponse(response *autopilot.PromptResponse) *api.AutopilotPromptResponse {
	if response == nil {
		return nil
	}
	resp := &api.AutopilotPromptResponse{
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

func toAPIAutopilotPendingTurnMessages(
	messages []autopilot.PendingTurnMessage,
) []api.AutopilotPendingTurnMessage {
	resp := make([]api.AutopilotPendingTurnMessage, 0, len(messages))
	for _, message := range messages {
		resp = append(resp, api.AutopilotPendingTurnMessage{
			CreatedAt: message.CreatedAt,
			Id:        message.ID,
			Kind:      message.Kind,
			Message:   message.Message,
		})
	}
	return resp
}

func toAPIAutopilotTask(task *autopilot.Task) api.AutopilotTask {
	if task == nil {
		return api.AutopilotTask{}
	}
	return api.AutopilotTask{
		CreatedAt:   ptrOf(task.CreatedAt),
		CreatedBy:   ptrOf(task.CreatedBy),
		Description: task.Description,
		DoneAt:      ptrOf(task.DoneAt),
		DoneBy:      ptrOf(task.DoneBy),
		Id:          task.ID,
		State:       api.AutopilotTaskState(task.State),
		UpdatedAt:   ptrOf(task.UpdatedAt),
		UpdatedBy:   ptrOf(task.UpdatedBy),
	}
}

func toAPIAutopilotTasks(tasks []autopilot.Task) []api.AutopilotTask {
	resp := make([]api.AutopilotTask, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]
		resp = append(resp, toAPIAutopilotTask(&task))
	}
	return resp
}

func toAPIAutopilotTaskTemplates(tasks []autopilot.TaskTemplate) []api.AutopilotTaskTemplate {
	resp := make([]api.AutopilotTaskTemplate, 0, len(tasks))
	for _, task := range tasks {
		resp = append(resp, api.AutopilotTaskTemplate{
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

func toAPIAutopilotRunRef(ref *exec.DAGRunRef) *api.AutopilotRunRef {
	if ref == nil {
		return nil
	}
	return &api.AutopilotRunRef{
		Id:   ref.ID,
		Name: ref.Name,
	}
}

func toAPIAutopilotRunSummary(run *autopilot.RunSummary) *api.AutopilotRunSummary {
	if run == nil {
		return nil
	}
	resp := &api.AutopilotRunSummary{
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
