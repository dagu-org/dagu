package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/service/audit"
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
		return nil, errFailedToLoadAgentConfig
	}

	applyAgentConfigUpdates(cfg, request.Body)

	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return nil, errFailedToSaveAgentConfig
	}

	a.logAuditEntry(ctx, audit.CategoryAgent, "agent_config_update", buildAgentConfigChanges(request.Body))

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

func buildAgentConfigChanges(update *api.UpdateAgentConfigRequest) map[string]any {
	changes := make(map[string]any)
	if update.Enabled != nil {
		changes["enabled"] = *update.Enabled
	}
	if llmChanges := buildLLMConfigChanges(update.Llm); len(llmChanges) > 0 {
		changes["llm"] = llmChanges
	}
	return changes
}

func buildLLMConfigChanges(update *api.UpdateAgentLLMConfig) map[string]any {
	if update == nil {
		return nil
	}
	changes := make(map[string]any)
	if update.Provider != nil {
		changes["provider"] = *update.Provider
	}
	if update.Model != nil {
		changes["model"] = *update.Model
	}
	if update.ApiKey != nil {
		changes["api_key_changed"] = true
	}
	if update.BaseUrl != nil {
		changes["base_url"] = *update.BaseUrl
	}
	return changes
}
