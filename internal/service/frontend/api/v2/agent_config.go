package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/persis/fileagentconfig"
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

	a.logAgentAudit(ctx, "agent_config_update", buildAgentConfigChanges(request.Body))

	return api.UpdateAgentConfig200JSONResponse(toAgentConfigResponse(cfg)), nil
}

func (a *API) requireAgentConfigManagement() error {
	if a.agentConfigStore == nil {
		return errAgentConfigNotAvailable
	}
	return nil
}

func (a *API) logAgentAudit(ctx context.Context, action string, details map[string]any) {
	if a.auditService == nil {
		return
	}
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok || currentUser == nil {
		return
	}
	clientIP, _ := auth.ClientIPFromContext(ctx)
	detailsJSON, _ := json.Marshal(details)
	entry := audit.NewEntry(audit.CategoryAgent, action, currentUser.ID, currentUser.Username).
		WithDetails(string(detailsJSON)).
		WithIPAddress(clientIP)
	_ = a.auditService.Log(ctx, entry)
}

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

func applyAgentConfigUpdates(cfg *fileagentconfig.AgentConfig, update *api.UpdateAgentConfigRequest) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.Llm == nil {
		return
	}
	if update.Llm.Provider != nil {
		cfg.LLM.Provider = *update.Llm.Provider
	}
	if update.Llm.Model != nil {
		cfg.LLM.Model = *update.Llm.Model
	}
	if update.Llm.ApiKey != nil {
		cfg.LLM.APIKey = *update.Llm.ApiKey
	}
	if update.Llm.BaseUrl != nil {
		cfg.LLM.BaseURL = *update.Llm.BaseUrl
	}
}

func buildAgentConfigChanges(update *api.UpdateAgentConfigRequest) map[string]any {
	changes := make(map[string]any)
	if update.Enabled != nil {
		changes["enabled"] = *update.Enabled
	}
	if update.Llm == nil {
		return changes
	}
	llmChanges := make(map[string]any)
	if update.Llm.Provider != nil {
		llmChanges["provider"] = *update.Llm.Provider
	}
	if update.Llm.Model != nil {
		llmChanges["model"] = *update.Llm.Model
	}
	if update.Llm.ApiKey != nil {
		llmChanges["api_key_changed"] = true
	}
	if update.Llm.BaseUrl != nil {
		llmChanges["base_url"] = *update.Llm.BaseUrl
	}
	if len(llmChanges) > 0 {
		changes["llm"] = llmChanges
	}
	return changes
}
