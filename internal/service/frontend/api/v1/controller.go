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
	"github.com/dagucloud/dagu/internal/controller"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/audit"
	"gopkg.in/yaml.v3"
)

const (
	auditActionControllerDocumentUpdate = "document_update"
	auditActionControllerDocumentDelete = "document_delete"
)

func (a *API) ListController(ctx context.Context, _ api.ListControllerRequestObject) (api.ListControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentControllerStatus(ctx)
	items, err := a.controllerService.List(ctx)
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	resp := make([]api.ControllerSummary, 0, len(items))
	for _, item := range items {
		if !a.canAccessWorkspace(ctx, controllerWorkspaceNameFromLabels(item.Labels)) {
			continue
		}
		summary := toAPIControllerSummary(item)
		summary.ControllerStatus = &controllerStatus
		resp = append(resp, summary)
	}
	return api.ListController200JSONResponse{Controller: resp}, nil
}

func (a *API) GetController(ctx context.Context, request api.GetControllerRequestObject) (api.GetControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	controllerStatus := a.currentControllerStatus(ctx)
	item, err := a.controllerService.Detail(ctx, string(request.Name))
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	if err := a.requireWorkspaceVisible(ctx, controllerWorkspaceNameFromDetail(item)); err != nil {
		return nil, err
	}
	resp := toAPIControllerDetail(item)
	resp.ControllerStatus = &controllerStatus
	return api.GetController200JSONResponse(resp), nil
}

func (a *API) GetControllerSpec(ctx context.Context, request api.GetControllerSpecRequestObject) (api.GetControllerSpecResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if err := a.requireControllerVisible(ctx, string(request.Name)); err != nil {
		return nil, err
	}
	spec, err := a.controllerService.GetSpec(ctx, string(request.Name))
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	return api.GetControllerSpec200JSONResponse{Spec: spec}, nil
}

func (a *API) GetControllerDocument(ctx context.Context, request api.GetControllerDocumentRequestObject) (api.GetControllerDocumentResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if err := a.requireControllerDocumentStore(); err != nil {
		return nil, err
	}
	if err := a.requireControllerVisible(ctx, string(request.Name)); err != nil {
		return nil, err
	}
	item, err := a.controllerService.GetDocument(ctx, string(request.Name), string(request.Document))
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	return api.GetControllerDocument200JSONResponse(toAPIControllerDocument(item)), nil
}

func (a *API) UpdateControllerDocument(ctx context.Context, request api.UpdateControllerDocumentRequestObject) (api.UpdateControllerDocumentResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if err := a.requireControllerDocumentStore(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireControllerDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	document := string(request.Document)
	item, err := a.controllerService.SaveDocument(ctx, name, document, request.Body.Content)
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, auditActionControllerDocumentUpdate, map[string]any{
		"name":     name,
		"document": document,
	})
	return api.UpdateControllerDocument200JSONResponse(toAPIControllerDocument(item)), nil
}

func (a *API) DeleteControllerDocument(ctx context.Context, request api.DeleteControllerDocumentRequestObject) (api.DeleteControllerDocumentResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if err := a.requireControllerDocumentStore(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	document := string(request.Document)
	if err := a.controllerService.DeleteDocument(ctx, name, document); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, auditActionControllerDocumentDelete, map[string]any{
		"name":     name,
		"document": document,
	})
	return api.DeleteControllerDocument204Response{}, nil
}

func (a *API) PutControllerSpec(ctx context.Context, request api.PutControllerSpecRequestObject) (api.PutControllerSpecResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireControllerSpecWrite(ctx, name, request.Body.Spec); err != nil {
		return nil, err
	}
	if err := a.controllerService.PutSpec(ctx, name, request.Body.Spec); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "spec_upsert", map[string]any{"name": name})
	return api.PutControllerSpec204Response{}, nil
}

func (a *API) DeleteController(ctx context.Context, request api.DeleteControllerRequestObject) (api.DeleteControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	if err := a.controllerService.Delete(ctx, name); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "delete", map[string]any{"name": name})
	return api.DeleteController204Response{}, nil
}

func (a *API) RenameController(ctx context.Context, request api.RenameControllerRequestObject) (api.RenameControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireControllerDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	body := controller.RenameRequest{
		NewName:     request.Body.NewName,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.controllerService.Rename(ctx, name, body); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "rename", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.RenameController204Response{}, nil
}

func (a *API) DuplicateController(ctx context.Context, request api.DuplicateControllerRequestObject) (api.DuplicateControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	name := string(request.Name)
	if err := a.requireControllerDAGWrite(ctx, name); err != nil {
		return nil, err
	}
	body := controller.DuplicateRequest{NewName: request.Body.NewName}
	if err := a.controllerService.Duplicate(ctx, name, body); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "duplicate", map[string]any{
		"name":     name,
		"new_name": body.NewName,
	})
	return api.DuplicateController204Response{}, nil
}

func (a *API) ResetController(ctx context.Context, request api.ResetControllerRequestObject) (api.ResetControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	if err := a.controllerService.ResetState(ctx, name); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "reset", map[string]any{"name": name})
	return api.ResetController204Response{}, nil
}

func (a *API) StartController(ctx context.Context, request api.StartControllerRequestObject) (api.StartControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	body := controller.StartRequest{
		RequestedBy: a.currentUsername(ctx),
	}
	if request.Body != nil && request.Body.Instruction != nil {
		body.Instruction = *request.Body.Instruction
	}
	if err := a.controllerService.RequestStart(ctx, name, body); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "start", map[string]any{
		"name":        name,
		"instruction": body.Instruction,
	})
	return api.StartController204Response{}, nil
}

func (a *API) PauseController(ctx context.Context, request api.PauseControllerRequestObject) (api.PauseControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	if err := a.controllerService.Pause(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "pause", map[string]any{"name": name})
	return api.PauseController204Response{}, nil
}

func (a *API) ResumeController(ctx context.Context, request api.ResumeControllerRequestObject) (api.ResumeControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	if err := a.controllerService.Resume(ctx, name, a.currentUsername(ctx)); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "resume", map[string]any{"name": name})
	return api.ResumeController204Response{}, nil
}

func (a *API) CreateControllerTask(ctx context.Context, request api.CreateControllerTaskRequestObject) (api.CreateControllerTaskResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	task, err := a.controllerService.CreateTask(ctx, name, controller.CreateTaskRequest{
		Description: request.Body.Description,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "task_create", map[string]any{
		"name": name,
		"id":   task.ID,
	})
	return api.CreateControllerTask200JSONResponse(toAPIControllerTask(task)), nil
}

func (a *API) UpdateControllerTask(ctx context.Context, request api.UpdateControllerTaskRequestObject) (api.UpdateControllerTaskResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	task, err := a.controllerService.UpdateTask(ctx, name, request.TaskId, controller.UpdateTaskRequest{
		Description: request.Body.Description,
		Done:        request.Body.Done,
		RequestedBy: a.currentUsername(ctx),
	})
	if err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "task_update", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.UpdateControllerTask200JSONResponse(toAPIControllerTask(task)), nil
}

func (a *API) DeleteControllerTask(ctx context.Context, request api.DeleteControllerTaskRequestObject) (api.DeleteControllerTaskResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.controllerService.DeleteTask(ctx, name, request.TaskId, a.currentUsername(ctx)); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "task_delete", map[string]any{
		"name": name,
		"id":   request.TaskId,
	})
	return api.DeleteControllerTask204Response{}, nil
}

func (a *API) ReorderControllerTasks(ctx context.Context, request api.ReorderControllerTasksRequestObject) (api.ReorderControllerTasksResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	if err := a.controllerService.ReorderTasks(ctx, name, controller.ReorderTasksRequest{
		TaskIDs:     append([]string(nil), request.Body.TaskIds...),
		RequestedBy: a.currentUsername(ctx),
	}); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "task_reorder", map[string]any{
		"name": name,
	})
	return api.ReorderControllerTasks204Response{}, nil
}

func (a *API) MessageController(ctx context.Context, request api.MessageControllerRequestObject) (api.MessageControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	body := controller.OperatorMessageRequest{
		Message:     request.Body.Message,
		RequestedBy: a.currentUsername(ctx),
	}
	if err := a.controllerService.SubmitOperatorMessage(ctx, name, body); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "message", map[string]any{"name": name})
	return api.MessageController204Response{}, nil
}

func (a *API) RespondController(ctx context.Context, request api.RespondControllerRequestObject) (api.RespondControllerResponseObject, error) {
	if err := a.requireControllerService(); err != nil {
		return nil, err
	}
	name := string(request.Name)
	if err := a.requireControllerExecute(ctx, name); err != nil {
		return nil, err
	}
	if err := a.requireReadyControllerStatus(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	body := controller.HumanResponseRequest{
		PromptID:          request.Body.PromptId,
		SelectedOptionIDs: append([]string(nil), valueOf(request.Body.SelectedOptionIds)...),
		FreeTextResponse:  valueOf(request.Body.FreeTextResponse),
	}
	if err := a.controllerService.SubmitHumanResponse(ctx, name, body); err != nil {
		return nil, toControllerAPIError(err)
	}
	a.logAudit(ctx, audit.CategoryController, "respond", map[string]any{
		"name":      name,
		"prompt_id": body.PromptID,
	})
	return api.RespondController204Response{}, nil
}

func controllerWorkspaceNameFromLabels(labels []string) string {
	workspaceName, state := exec.WorkspaceLabelFromLabels(core.NewLabels(labels))
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

func controllerWorkspaceNameFromDefinition(def *controller.Definition) string {
	if def == nil {
		return ""
	}
	return controllerWorkspaceNameFromLabels(def.Labels)
}

func controllerWorkspaceNameFromDetail(detail *controller.Detail) string {
	if detail == nil {
		return ""
	}
	return controllerWorkspaceNameFromDefinition(detail.Definition)
}

func controllerWorkspaceNameFromSpec(spec string) (string, error) {
	var def controller.Definition
	if err := yaml.Unmarshal([]byte(spec), &def); err != nil {
		return "", err
	}
	return controllerWorkspaceNameFromDefinition(&def), nil
}

func (a *API) controllerWorkspaceName(ctx context.Context, name string) (string, error) {
	spec, err := a.controllerService.GetSpec(ctx, name)
	if err != nil {
		return "", err
	}
	workspaceName, err := controllerWorkspaceNameFromSpec(spec)
	if err != nil {
		return "", nil
	}
	return workspaceName, nil
}

func (a *API) requireControllerVisible(ctx context.Context, name string) error {
	workspaceName, err := a.controllerWorkspaceName(ctx, name)
	if err != nil {
		return toControllerAPIError(err)
	}
	return a.requireWorkspaceVisible(ctx, workspaceName)
}

func (a *API) requireControllerDAGWrite(ctx context.Context, name string) error {
	workspaceName, err := a.controllerWorkspaceName(ctx, name)
	if err != nil {
		return toControllerAPIError(err)
	}
	return a.requireDAGWriteForWorkspace(ctx, workspaceName)
}

func (a *API) requireControllerExecute(ctx context.Context, name string) error {
	workspaceName, err := a.controllerWorkspaceName(ctx, name)
	if err != nil {
		return toControllerAPIError(err)
	}
	return a.requireExecuteForWorkspace(ctx, workspaceName)
}

func (a *API) requireControllerSpecWrite(ctx context.Context, name, spec string) error {
	currentWorkspaceName, err := a.controllerWorkspaceName(ctx, name)
	if err == nil {
		if err := a.requireDAGWriteForWorkspace(ctx, currentWorkspaceName); err != nil {
			return err
		}
	} else if !errors.Is(err, exec.ErrDAGNotFound) && !errors.Is(err, os.ErrNotExist) {
		return toControllerAPIError(err)
	}

	nextWorkspaceName, err := controllerWorkspaceNameFromSpec(spec)
	if err != nil {
		return a.requireDAGWrite(ctx)
	}
	return a.requireDAGWriteForWorkspace(ctx, nextWorkspaceName)
}

func (a *API) requireControllerService() error {
	if a.controllerService == nil {
		return &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Controller service is not available",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	return nil
}

func (a *API) requireReadyControllerStatus(ctx context.Context) error {
	status := a.currentControllerStatus(ctx)
	if status.State == api.ControllerStatusStateReady {
		return nil
	}
	message := "No active scheduler with a ready Controller is available."
	if status.Message != nil && *status.Message != "" {
		message = *status.Message
	}
	return &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

func (a *API) requireControllerDocumentStore() error {
	if _, ok := a.agentMemoryStore.(agent.ControllerDocumentStore); !ok {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Controller document management is not available",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

func (a *API) currentControllerStatus(ctx context.Context) api.ControllerStatus {
	status := normalizeControllerStatus(nil)
	if a.serviceRegistry == nil {
		status.State = exec.ControllerStatusStateUnavailable
		status.Message = "Service registry is not configured"
		return toAPIControllerStatus(status)
	}

	members, err := a.serviceRegistry.GetServiceMembers(ctx, exec.ServiceNameScheduler)
	if err != nil {
		status.State = exec.ControllerStatusStateUnavailable
		status.Message = "Failed to retrieve scheduler status"
		return toAPIControllerStatus(status)
	}

	var fallback exec.ControllerStatusInfo
	hasFallback := false
	for _, member := range members {
		if member.Status != exec.ServiceStatusActive {
			continue
		}
		normalized := normalizeControllerStatus(member.ControllerStatus)
		if normalized.State == exec.ControllerStatusStateReady {
			return toAPIControllerStatus(normalized)
		}
		if !hasFallback {
			fallback = normalized
			hasFallback = true
		}
	}
	if hasFallback {
		return toAPIControllerStatus(fallback)
	}

	status.State = exec.ControllerStatusStateUnavailable
	status.Message = "No active scheduler with a ready Controller is available."
	return toAPIControllerStatus(status)
}

func normalizeControllerStatus(info *exec.ControllerStatusInfo) exec.ControllerStatusInfo {
	if info == nil || info.State == "" {
		return exec.ControllerStatusInfo{
			State:   exec.ControllerStatusStateUnknown,
			Message: "Scheduler controller readiness is unknown",
		}
	}
	normalized := *info
	if normalized.Message == "" {
		switch normalized.State {
		case exec.ControllerStatusStateReady:
		case exec.ControllerStatusStateDisabled:
			normalized.Message = "Controller is disabled in agent settings"
		case exec.ControllerStatusStateUnavailable:
			normalized.Message = "Controller is unavailable"
		case exec.ControllerStatusStateUnknown:
			normalized.Message = "Scheduler controller readiness is unknown"
		}
	}
	return normalized
}

func toAPIControllerStatus(info exec.ControllerStatusInfo) api.ControllerStatus {
	status := api.ControllerStatus{
		State: api.ControllerStatusState(info.State),
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

func toControllerAPIError(err error) error {
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

func toAPIControllerSummary(item controller.Summary) api.ControllerSummary {
	return api.ControllerSummary{
		Busy: func() *bool {
			v := item.Busy
			return &v
		}(),
		CurrentRun:    toAPIControllerRunSummary(item.CurrentRun),
		ClonedFrom:    ptrOf(item.ClonedFrom),
		Description:   ptrOf(item.Description),
		Disabled:      ptrOf(item.Disabled),
		DisplayStatus: ptrOf(api.ControllerDisplayStatus(item.DisplayStatus)),
		DoneTaskCount: ptrOf(item.DoneTaskCount),
		Goal:          ptrOf(item.Goal),
		IconUrl:       ptrOf(item.IconURL),
		Instruction:   ptrOf(item.Instruction),
		Kind:          api.ControllerKind(item.Kind),
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
		State:               api.ControllerLifecycleState(item.State),
		Labels: func() *[]string {
			if len(item.Labels) == 0 {
				return nil
			}
			labels := append([]string(nil), item.Labels...)
			return &labels
		}(),
	}
}

func toAPIControllerDocument(item *controller.Document) api.ControllerDocumentResponse {
	if item == nil {
		return api.ControllerDocumentResponse{}
	}
	return api.ControllerDocumentResponse{
		Content:  item.Content,
		Document: api.ControllerDocument(item.Document),
		Name:     item.Name,
		Path:     item.Path,
	}
}

func toAPIControllerDetail(item *controller.Detail) api.ControllerDetailResponse {
	if item == nil {
		return api.ControllerDetailResponse{
			Workflows:     []api.ControllerWorkflowInfo{},
			TaskTemplates: &[]api.ControllerTaskTemplate{},
		}
	}
	resp := api.ControllerDetailResponse{
		Workflows:  toAPIControllerWorkflowInfos(item.Workflows),
		CurrentRun: toAPIControllerRunSummary(item.CurrentRun),
		Definition: toAPIControllerDefinition(item.Definition),
		State:      toAPIControllerState(item.Definition, item.State),
	}
	taskTemplates := toAPIControllerTaskTemplates(item.TaskTemplates)
	resp.TaskTemplates = &taskTemplates
	if len(item.RecentRuns) > 0 {
		runs := make([]api.ControllerRunSummary, 0, len(item.RecentRuns))
		for _, run := range item.RecentRuns {
			apiRun := toAPIControllerRunSummary(&run)
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

func toAPIControllerDefinition(def *controller.Definition) api.ControllerDefinition {
	if def == nil {
		return api.ControllerDefinition{}
	}
	resp := api.ControllerDefinition{
		Description:   ptrOf(def.Description),
		Disabled:      ptrOf(def.Disabled),
		Goal:          ptrOf(def.Goal),
		IconUrl:       ptrOf(def.IconURL),
		Kind:          api.ControllerKind(def.Kind),
		Name:          def.Name,
		Nickname:      ptrOf(def.Nickname),
		ClonedFrom:    ptrOf(def.ClonedFrom),
		Workflows:     toAPIControllerWorkflows(def.Workflows),
		ResetOnFinish: ptrOf(def.ResetOnFinish),
		Trigger:       toAPIControllerTrigger(def.Trigger),
		Labels: func() *[]string {
			if len(def.Labels) == 0 {
				return nil
			}
			labels := append([]string(nil), def.Labels...)
			return &labels
		}(),
	}
	if agentConfig := toAPIControllerAgentConfig(def.Agent); agentConfig != nil {
		resp.Agent = agentConfig
	}
	return resp
}

func toAPIControllerTrigger(trigger controller.Trigger) api.ControllerTrigger {
	resp := api.ControllerTrigger{
		Type: api.ControllerTriggerType(trigger.Type),
	}
	if len(trigger.Schedules) > 0 {
		schedules := make([]string, 0, len(trigger.Schedules))
		for _, item := range trigger.Schedules {
			if item.Expression != "" {
				schedules = append(schedules, item.Expression)
			}
		}
		if len(schedules) > 0 {
			resp.Schedules = &schedules
		}
	}
	if trigger.Prompt != "" {
		resp.Prompt = ptrOf(trigger.Prompt)
	}
	return resp
}

func toAPIControllerAgentConfig(cfg controller.AgentConfig) *api.ControllerAgentConfig {
	resp := &api.ControllerAgentConfig{
		Model:    ptrOf(cfg.Model),
		SafeMode: new(cfg.SafeMode),
		Soul:     ptrOf(cfg.Soul),
	}
	if resp.Model == nil && resp.SafeMode == nil && resp.Soul == nil {
		return nil
	}
	return resp
}

func toAPIControllerWorkflows(workflows controller.Workflows) *api.ControllerWorkflows {
	resp := &api.ControllerWorkflows{}
	if len(workflows.Names) > 0 {
		names := append([]string(nil), workflows.Names...)
		resp.Names = &names
	}
	if len(workflows.Labels) > 0 {
		labels := append([]string(nil), workflows.Labels...)
		resp.Labels = &labels
	}
	if resp.Names == nil && resp.Labels == nil {
		return nil
	}
	return resp
}

func toAPIControllerWorkflowInfos(items []controller.WorkflowInfo) []api.ControllerWorkflowInfo {
	if len(items) == 0 {
		return []api.ControllerWorkflowInfo{}
	}
	resp := make([]api.ControllerWorkflowInfo, 0, len(items))
	for _, item := range items {
		apiItem := api.ControllerWorkflowInfo{
			Description: ptrOf(item.Description),
			Name:        item.Name,
		}
		if len(item.Labels) > 0 {
			labels := append([]string(nil), item.Labels...)
			apiItem.Labels = &labels
		}
		resp = append(resp, apiItem)
	}
	return resp
}

func toAPIControllerState(def *controller.Definition, state *controller.State) api.ControllerState {
	if state == nil {
		return api.ControllerState{}
	}
	view := controller.DeriveView(def, state)
	resp := api.ControllerState{
		ActivatedAt: ptrOf(state.ActivatedAt),
		ActivatedBy: ptrOf(state.ActivatedBy),
		Busy: func() *bool {
			v := view.Busy
			return &v
		}(),
		CurrentCycleId:       ptrOf(state.CurrentCycleID),
		CurrentRunRef:        toAPIControllerRunRef(state.CurrentRunRef),
		DisplayStatus:        ptrOf(api.ControllerDisplayStatus(view.DisplayStatus)),
		FinishedAt:           ptrOf(state.FinishedAt),
		Instruction:          ptrOf(state.Instruction),
		InstructionUpdatedAt: ptrOf(state.InstructionUpdatedAt),
		InstructionUpdatedBy: ptrOf(state.InstructionUpdatedBy),
		LastError:            ptrOf(state.LastError),
		LastRunRef:           toAPIControllerRunRef(state.LastRunRef),
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
		PendingPrompt:    toAPIControllerPrompt(state.PendingPrompt),
		PendingResponse:  toAPIControllerPromptResponse(state.PendingResponse),
		SessionId:        ptrOf(state.SessionID),
		StartRequestedAt: ptrOf(state.StartRequestedAt),
		State:            api.ControllerLifecycleState(state.State),
		WaitingReason:    toAPIControllerWaitingReason(state.WaitingReason),
	}
	if len(state.Tasks) > 0 {
		tasks := toAPIControllerTasks(state.Tasks)
		resp.Tasks = &tasks
	}
	if len(state.PendingTurnMessages) > 0 {
		messages := toAPIControllerPendingTurnMessages(state.PendingTurnMessages)
		resp.PendingTurnMessages = &messages
	}
	return resp
}

func toAPIControllerWaitingReason(reason controller.WaitingReason) *api.ControllerWaitingReason {
	if reason == "" {
		return nil
	}
	apiReason := api.ControllerWaitingReason(reason)
	return &apiReason
}

func toAPIControllerPrompt(prompt *controller.Prompt) *api.ControllerPrompt {
	if prompt == nil {
		return nil
	}
	resp := &api.ControllerPrompt{
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

func toAPIControllerPromptResponse(response *controller.PromptResponse) *api.ControllerPromptResponse {
	if response == nil {
		return nil
	}
	resp := &api.ControllerPromptResponse{
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

func toAPIControllerPendingTurnMessages(
	messages []controller.PendingTurnMessage,
) []api.ControllerPendingTurnMessage {
	resp := make([]api.ControllerPendingTurnMessage, 0, len(messages))
	for _, message := range messages {
		resp = append(resp, api.ControllerPendingTurnMessage{
			CreatedAt: message.CreatedAt,
			Id:        message.ID,
			Kind:      message.Kind,
			Message:   message.Message,
		})
	}
	return resp
}

func toAPIControllerTask(task *controller.Task) api.ControllerTask {
	if task == nil {
		return api.ControllerTask{}
	}
	return api.ControllerTask{
		CreatedAt:   ptrOf(task.CreatedAt),
		CreatedBy:   ptrOf(task.CreatedBy),
		Description: task.Description,
		DoneAt:      ptrOf(task.DoneAt),
		DoneBy:      ptrOf(task.DoneBy),
		Id:          task.ID,
		State:       api.ControllerTaskState(task.State),
		UpdatedAt:   ptrOf(task.UpdatedAt),
		UpdatedBy:   ptrOf(task.UpdatedBy),
	}
}

func toAPIControllerTasks(tasks []controller.Task) []api.ControllerTask {
	resp := make([]api.ControllerTask, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]
		resp = append(resp, toAPIControllerTask(&task))
	}
	return resp
}

func toAPIControllerTaskTemplates(tasks []controller.TaskTemplate) []api.ControllerTaskTemplate {
	resp := make([]api.ControllerTaskTemplate, 0, len(tasks))
	for _, task := range tasks {
		resp = append(resp, api.ControllerTaskTemplate{
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

func toAPIControllerRunRef(ref *exec.DAGRunRef) *api.ControllerRunRef {
	if ref == nil {
		return nil
	}
	return &api.ControllerRunRef{
		Id:   ref.ID,
		Name: ref.Name,
	}
}

func toAPIControllerRunSummary(run *controller.RunSummary) *api.ControllerRunSummary {
	if run == nil {
		return nil
	}
	resp := &api.ControllerRunSummary{
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
