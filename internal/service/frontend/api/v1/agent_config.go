package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionAgentConfigUpdate = "agent_config_update"
	auditFieldEnabled            = "enabled"
	auditFieldDefaultModelID     = "default_model_id"
)

var (
	errAgentConfigNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent configuration management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errFailedToLoadAgentConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to load agent configuration",
		HTTPStatus: http.StatusInternalServerError,
	}

	errFailedToSaveAgentConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to save agent configuration",
		HTTPStatus: http.StatusInternalServerError,
	}

	errInvalidRequestBody = &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    "Invalid request body",
		HTTPStatus: http.StatusBadRequest,
	}
)

// GetAgentConfig returns the current agent configuration. Requires admin role.
func (a *API) GetAgentConfig(ctx context.Context, _ api.GetAgentConfigRequestObject) (api.GetAgentConfigResponseObject, error) {
	if err := a.requireAgentConfigManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load agent config", tag.Error(err))
		return nil, errFailedToLoadAgentConfig
	}

	return api.GetAgentConfig200JSONResponse(toAgentConfigResponse(cfg)), nil
}

// UpdateAgentConfig updates the agent configuration. Requires admin role.
func (a *API) UpdateAgentConfig(ctx context.Context, request api.UpdateAgentConfigRequestObject) (api.UpdateAgentConfigResponseObject, error) {
	if err := a.requireAgentConfigManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load agent config", tag.Error(err))
		return nil, errFailedToLoadAgentConfig
	}

	applyAgentConfigUpdates(cfg, request.Body)

	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		logger.Error(ctx, "Failed to save agent config", tag.Error(err))
		return nil, errFailedToSaveAgentConfig
	}

	a.logAuditEntry(ctx, audit.CategoryAgent, auditActionAgentConfigUpdate, buildAgentConfigChanges(request.Body))

	return api.UpdateAgentConfig200JSONResponse(toAgentConfigResponse(cfg)), nil
}

func (a *API) requireAgentConfigManagement() error {
	if a.agentConfigStore == nil {
		return errAgentConfigNotAvailable
	}
	return nil
}

func toAgentConfigResponse(cfg *agent.Config) api.AgentConfigResponse {
	return api.AgentConfigResponse{
		Enabled:        &cfg.Enabled,
		DefaultModelId: ptrOf(cfg.DefaultModelID),
	}
}

// applyAgentConfigUpdates applies non-nil fields from the update request to the agent configuration.
func applyAgentConfigUpdates(cfg *agent.Config, update *api.UpdateAgentConfigRequest) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.DefaultModelId != nil {
		cfg.DefaultModelID = *update.DefaultModelId
	}
}

// buildAgentConfigChanges constructs a map of changed fields for audit logging.
func buildAgentConfigChanges(update *api.UpdateAgentConfigRequest) map[string]any {
	changes := make(map[string]any)
	if update.Enabled != nil {
		changes[auditFieldEnabled] = *update.Enabled
	}
	if update.DefaultModelId != nil {
		changes[auditFieldDefaultModelID] = *update.DefaultModelId
	}
	return changes
}
