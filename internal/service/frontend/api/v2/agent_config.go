package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionAgentConfigUpdate = "agent_config_update"
	auditFieldEnabled            = "enabled"
	auditFieldLLM                = "llm"
	auditFieldProvider           = "provider"
	auditFieldModel              = "model"
	auditFieldAPIKeyChanged      = "api_key_changed"
	auditFieldBaseURL            = "base_url"
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
		Enabled: ptrOf(cfg.Enabled),
		Llm: &api.AgentLLMConfig{
			Provider:         ptrOf(cfg.LLM.Provider),
			Model:            ptrOf(cfg.LLM.Model),
			ApiKeyConfigured: ptrOf(cfg.LLM.APIKey != ""),
			BaseUrl:          ptrOf(cfg.LLM.BaseURL),
		},
	}
}

// applyAgentConfigUpdates applies non-nil fields from the update request to the agent configuration.
// Only fields present in the update are modified, allowing partial updates.
func applyAgentConfigUpdates(cfg *agent.Config, update *api.UpdateAgentConfigRequest) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	applyLLMConfigUpdates(&cfg.LLM, update.Llm)
}

func applyLLMConfigUpdates(cfg *agent.LLMConfig, update *api.UpdateAgentLLMConfig) {
	if update == nil {
		return
	}
	if update.Provider != nil {
		cfg.Provider = *update.Provider
	}
	if update.Model != nil {
		cfg.Model = *update.Model
	}
	if update.ApiKey != nil {
		cfg.APIKey = *update.ApiKey
	}
	if update.BaseUrl != nil {
		cfg.BaseURL = *update.BaseUrl
	}
}

// buildAgentConfigChanges constructs a map of changed fields for audit logging.
// Only non-nil fields from the update are included in the returned map.
func buildAgentConfigChanges(update *api.UpdateAgentConfigRequest) map[string]any {
	changes := make(map[string]any)
	if update.Enabled != nil {
		changes[auditFieldEnabled] = *update.Enabled
	}
	if llmChanges := buildLLMConfigChanges(update.Llm); len(llmChanges) > 0 {
		changes[auditFieldLLM] = llmChanges
	}
	return changes
}

func buildLLMConfigChanges(update *api.UpdateAgentLLMConfig) map[string]any {
	if update == nil {
		return nil
	}
	changes := make(map[string]any)
	if update.Provider != nil {
		changes[auditFieldProvider] = *update.Provider
	}
	if update.Model != nil {
		changes[auditFieldModel] = *update.Model
	}
	if update.ApiKey != nil {
		changes[auditFieldAPIKeyChanged] = true
	}
	if update.BaseUrl != nil {
		changes[auditFieldBaseURL] = *update.BaseUrl
	}
	return changes
}
