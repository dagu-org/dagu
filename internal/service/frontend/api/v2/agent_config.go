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
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to load agent configuration",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.GetAgentConfig200JSONResponse{
		Enabled: ptrOf(cfg.Enabled),
		Llm: &api.AgentLLMConfig{
			Provider:         ptrOf(cfg.LLM.Provider),
			Model:            ptrOf(cfg.LLM.Model),
			ApiKeyConfigured: ptrOf(cfg.LLM.APIKey != ""),
			BaseUrl:          ptrOf(cfg.LLM.BaseURL),
		},
	}, nil
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
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid request body",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	// Load existing config
	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to load agent configuration",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	// Apply updates
	if request.Body.Enabled != nil {
		cfg.Enabled = *request.Body.Enabled
	}
	if request.Body.Llm != nil {
		if request.Body.Llm.Provider != nil {
			cfg.LLM.Provider = *request.Body.Llm.Provider
		}
		if request.Body.Llm.Model != nil {
			cfg.LLM.Model = *request.Body.Llm.Model
		}
		if request.Body.Llm.ApiKey != nil {
			cfg.LLM.APIKey = *request.Body.Llm.ApiKey
		}
		if request.Body.Llm.BaseUrl != nil {
			cfg.LLM.BaseURL = *request.Body.Llm.BaseUrl
		}
	}

	// Save config
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to save agent configuration",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	// Audit log the change
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		changes := make(map[string]any)
		if request.Body.Enabled != nil {
			changes["enabled"] = *request.Body.Enabled
		}
		if request.Body.Llm != nil {
			llmChanges := make(map[string]any)
			if request.Body.Llm.Provider != nil {
				llmChanges["provider"] = *request.Body.Llm.Provider
			}
			if request.Body.Llm.Model != nil {
				llmChanges["model"] = *request.Body.Llm.Model
			}
			if request.Body.Llm.ApiKey != nil {
				llmChanges["api_key_changed"] = true
			}
			if request.Body.Llm.BaseUrl != nil {
				llmChanges["base_url"] = *request.Body.Llm.BaseUrl
			}
			if len(llmChanges) > 0 {
				changes["llm"] = llmChanges
			}
		}
		details, err := json.Marshal(changes)
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryAgent, "agent_config_update", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	// Trigger agent reload if configured
	if a.agentReloader != nil {
		if err := a.agentReloader.ReloadAgent(ctx); err != nil {
			logger.Warn(ctx, "Failed to reload agent after config update", tag.Error(err))
		}
	}

	return api.UpdateAgentConfig200JSONResponse{
		Enabled: ptrOf(cfg.Enabled),
		Llm: &api.AgentLLMConfig{
			Provider:         ptrOf(cfg.LLM.Provider),
			Model:            ptrOf(cfg.LLM.Model),
			ApiKeyConfigured: ptrOf(cfg.LLM.APIKey != ""),
			BaseUrl:          ptrOf(cfg.LLM.BaseURL),
		},
	}, nil
}

// requireAgentConfigManagement checks if agent config management is available.
func (a *API) requireAgentConfigManagement() error {
	if a.agentConfigStore == nil {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Agent configuration management is not available",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}
