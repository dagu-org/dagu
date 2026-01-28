package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/persis/fileagentconfig"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// AgentConfigStore defines the interface for agent configuration storage.
type AgentConfigStore interface {
	Load(ctx context.Context) (*fileagentconfig.AgentConfig, error)
	Save(ctx context.Context, cfg *fileagentconfig.AgentConfig) error
}

// AgentReloader defines the interface for reloading the agent.
type AgentReloader interface {
	ReloadAgent(ctx context.Context) error
}

// GetAgentConfig returns the current agent configuration.
// Requires admin role.
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

// UpdateAgentConfig updates the agent configuration.
// Requires admin role.
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

	a.logAgentConfigUpdate(ctx, request.Body)
	a.reloadAgentIfConfigured(ctx)

	return api.UpdateAgentConfig200JSONResponse(toAgentConfigResponse(cfg)), nil
}

// requireAgentConfigManagement checks if agent config management is available.
func (a *API) requireAgentConfigManagement() error {
	if a.agentConfigStore == nil {
		return errAgentConfigNotAvailable
	}
	return nil
}

// Common errors for agent configuration operations.
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

// toAgentConfigResponse converts internal agent config to API response format.
func toAgentConfigResponse(cfg *fileagentconfig.AgentConfig) api.AgentConfigResponse {
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

// applyAgentConfigUpdates applies the update request fields to the config.
func applyAgentConfigUpdates(cfg *fileagentconfig.AgentConfig, update *api.UpdateAgentConfigRequest) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}

	if update.Llm != nil {
		applyLLMConfigUpdates(&cfg.LLM, update.Llm)
	}
}

// applyLLMConfigUpdates applies LLM-specific updates to the config.
func applyLLMConfigUpdates(cfg *fileagentconfig.AgentLLMConfig, update *api.UpdateAgentLLMConfig) {
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

// logAgentConfigUpdate logs the configuration change to the audit service.
func (a *API) logAgentConfigUpdate(ctx context.Context, update *api.UpdateAgentConfigRequest) {
	if a.auditService == nil {
		return
	}

	currentUser, _ := auth.UserFromContext(ctx)
	clientIP, _ := auth.ClientIPFromContext(ctx)

	details, err := json.Marshal(buildAgentConfigChanges(update))
	if err != nil {
		logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
		details = []byte("{}")
	}

	entry := audit.NewEntry(audit.CategoryAgent, "agent_config_update", currentUser.ID, currentUser.Username).
		WithDetails(string(details)).
		WithIPAddress(clientIP)
	_ = a.auditService.Log(ctx, entry)
}

// buildAgentConfigChanges builds a map of changes for audit logging.
func buildAgentConfigChanges(update *api.UpdateAgentConfigRequest) map[string]any {
	changes := make(map[string]any)

	if update.Enabled != nil {
		changes["enabled"] = *update.Enabled
	}

	if update.Llm != nil {
		llmChanges := buildLLMChanges(update.Llm)
		if len(llmChanges) > 0 {
			changes["llm"] = llmChanges
		}
	}

	return changes
}

// buildLLMChanges builds a map of LLM-related changes for audit logging.
func buildLLMChanges(llm *api.UpdateAgentLLMConfig) map[string]any {
	changes := make(map[string]any)

	if llm.Provider != nil {
		changes["provider"] = *llm.Provider
	}
	if llm.Model != nil {
		changes["model"] = *llm.Model
	}
	if llm.ApiKey != nil {
		changes["api_key_changed"] = true
	}
	if llm.BaseUrl != nil {
		changes["base_url"] = *llm.BaseUrl
	}

	return changes
}

// reloadAgentIfConfigured triggers agent reload if a reloader is configured.
func (a *API) reloadAgentIfConfigured(ctx context.Context) {
	if a.agentReloader == nil {
		return
	}

	if err := a.agentReloader.ReloadAgent(ctx); err != nil {
		logger.Warn(ctx, "Failed to reload agent after config update", tag.Error(err))
	}
}
