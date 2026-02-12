package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionMemoryUpdate = "agent_memory_update"
	auditActionMemoryDelete = "agent_memory_delete"
)

var (
	errAgentMemoryNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent memory management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errFailedToLoadMemory = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to load agent memory",
		HTTPStatus: http.StatusInternalServerError,
	}

	errFailedToSaveMemory = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to save agent memory",
		HTTPStatus: http.StatusInternalServerError,
	}

	errFailedToDeleteMemory = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to delete agent memory",
		HTTPStatus: http.StatusInternalServerError,
	}
)

// GetAgentMemory returns global memory and list of DAGs with memory. Requires admin role.
func (a *API) GetAgentMemory(ctx context.Context, _ api.GetAgentMemoryRequestObject) (api.GetAgentMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	globalContent, err := a.agentMemoryStore.LoadGlobalMemory(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load global memory", tag.Error(err))
		return nil, errFailedToLoadMemory
	}

	dagNames, err := a.agentMemoryStore.ListDAGMemories(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list DAG memories", tag.Error(err))
		return nil, errFailedToLoadMemory
	}

	return api.GetAgentMemory200JSONResponse(api.AgentMemoryResponse{
		GlobalMemory: &globalContent,
		DagMemories:  &dagNames,
		MemoryDir:    ptrOf(a.agentMemoryStore.MemoryDir()),
	}), nil
}

// UpdateAgentMemory updates the global MEMORY.md content. Requires admin role.
func (a *API) UpdateAgentMemory(ctx context.Context, request api.UpdateAgentMemoryRequestObject) (api.UpdateAgentMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	if err := a.agentMemoryStore.SaveGlobalMemory(ctx, request.Body.Content); err != nil {
		logger.Error(ctx, "Failed to save global memory", tag.Error(err))
		return nil, errFailedToSaveMemory
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionMemoryUpdate, map[string]any{"scope": "global"})

	return api.UpdateAgentMemory200Response{}, nil
}

// DeleteAgentMemory clears the global MEMORY.md. Requires admin role.
func (a *API) DeleteAgentMemory(ctx context.Context, _ api.DeleteAgentMemoryRequestObject) (api.DeleteAgentMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentMemoryStore.DeleteGlobalMemory(ctx); err != nil {
		logger.Error(ctx, "Failed to delete global memory", tag.Error(err))
		return nil, errFailedToDeleteMemory
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionMemoryDelete, map[string]any{"scope": "global"})

	return api.DeleteAgentMemory200Response{}, nil
}

// GetAgentDAGMemory returns memory content for a specific DAG. Requires admin role.
func (a *API) GetAgentDAGMemory(ctx context.Context, request api.GetAgentDAGMemoryRequestObject) (api.GetAgentDAGMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	content, err := a.agentMemoryStore.LoadDAGMemory(ctx, request.DagName)
	if err != nil {
		logger.Error(ctx, "Failed to load DAG memory", tag.Error(err))
		return nil, errFailedToLoadMemory
	}

	return api.GetAgentDAGMemory200JSONResponse(api.AgentDAGMemoryResponse{
		DagName: request.DagName,
		Content: content,
	}), nil
}

// UpdateAgentDAGMemory updates the memory content for a specific DAG. Requires admin role.
func (a *API) UpdateAgentDAGMemory(ctx context.Context, request api.UpdateAgentDAGMemoryRequestObject) (api.UpdateAgentDAGMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	if err := a.agentMemoryStore.SaveDAGMemory(ctx, request.DagName, request.Body.Content); err != nil {
		logger.Error(ctx, "Failed to save DAG memory", tag.Error(err))
		return nil, errFailedToSaveMemory
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionMemoryUpdate, map[string]any{
		"scope":   "dag",
		"dagName": request.DagName,
	})

	return api.UpdateAgentDAGMemory200Response{}, nil
}

// DeleteAgentDAGMemory clears the memory for a specific DAG. Requires admin role.
func (a *API) DeleteAgentDAGMemory(ctx context.Context, request api.DeleteAgentDAGMemoryRequestObject) (api.DeleteAgentDAGMemoryResponseObject, error) {
	if err := a.requireAgentMemoryManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentMemoryStore.DeleteDAGMemory(ctx, request.DagName); err != nil {
		logger.Error(ctx, "Failed to delete DAG memory", tag.Error(err))
		return nil, errFailedToDeleteMemory
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionMemoryDelete, map[string]any{
		"scope":   "dag",
		"dagName": request.DagName,
	})

	return api.DeleteAgentDAGMemory200Response{}, nil
}

func (a *API) requireAgentMemoryManagement() error {
	if a.agentMemoryStore == nil {
		return errAgentMemoryNotAvailable
	}
	return nil
}
