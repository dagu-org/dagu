package api

import (
	"context"
	"maps"
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
	auditFieldToolPolicy         = "tool_policy"
)

var (
	// ErrAgentConfigNotAvailable is returned when agent config management is disabled.
	ErrAgentConfigNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent configuration management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	// ErrFailedToLoadAgentConfig is returned when reading config fails.
	ErrFailedToLoadAgentConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to load agent configuration",
		HTTPStatus: http.StatusInternalServerError,
	}

	// ErrFailedToSaveAgentConfig is returned when writing config fails.
	ErrFailedToSaveAgentConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to save agent configuration",
		HTTPStatus: http.StatusInternalServerError,
	}

	// ErrInvalidRequestBody is returned when the request body is missing or invalid.
	ErrInvalidRequestBody = &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    "Invalid request body",
		HTTPStatus: http.StatusBadRequest,
	}

	// ErrInvalidToolPolicy is returned when tool policy validation fails.
	ErrInvalidToolPolicy = &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    "Invalid tool policy configuration",
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
		return nil, ErrFailedToLoadAgentConfig
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
		return nil, ErrInvalidRequestBody
	}

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load agent config", tag.Error(err))
		return nil, ErrFailedToLoadAgentConfig
	}

	if err := applyAgentConfigUpdates(cfg, request.Body); err != nil {
		return nil, ErrInvalidToolPolicy
	}

	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		logger.Error(ctx, "Failed to save agent config", tag.Error(err))
		return nil, ErrFailedToSaveAgentConfig
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionAgentConfigUpdate, buildAgentConfigChanges(request.Body))

	return api.UpdateAgentConfig200JSONResponse(toAgentConfigResponse(cfg)), nil
}

func (a *API) requireAgentConfigManagement() error {
	if a.agentConfigStore == nil {
		return ErrAgentConfigNotAvailable
	}
	return nil
}

func toAgentConfigResponse(cfg *agent.Config) api.AgentConfigResponse {
	return api.AgentConfigResponse{
		Enabled:        &cfg.Enabled,
		DefaultModelId: ptrOf(cfg.DefaultModelID),
		ToolPolicy:     toAPIToolPolicy(cfg.ToolPolicy),
	}
}

// applyAgentConfigUpdates applies non-nil fields from the update request to the agent configuration.
func applyAgentConfigUpdates(cfg *agent.Config, update *api.UpdateAgentConfigRequest) error {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.DefaultModelId != nil {
		cfg.DefaultModelID = *update.DefaultModelId
	}
	if update.ToolPolicy != nil {
		policy := toInternalToolPolicy(*update.ToolPolicy)
		if err := agent.ValidateToolPolicy(policy); err != nil {
			return err
		}
		cfg.ToolPolicy = agent.ResolveToolPolicy(policy)
	}
	return nil
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
	if update.ToolPolicy != nil {
		changes[auditFieldToolPolicy] = update.ToolPolicy
	}
	return changes
}

func toAPIToolPolicy(policy agent.ToolPolicyConfig) *api.AgentToolPolicy {
	resolved := agent.ResolveToolPolicy(policy)
	tools := make(map[string]bool, len(resolved.Tools))
	maps.Copy(tools, resolved.Tools)

	rules := make([]api.AgentBashRule, 0, len(resolved.Bash.Rules))
	for _, rule := range resolved.Bash.Rules {
		r := api.AgentBashRule{
			Name:    ptrOf(rule.Name),
			Pattern: rule.Pattern,
			Action:  api.AgentBashRuleAction(rule.Action),
		}
		if rule.Enabled != nil {
			r.Enabled = rule.Enabled
		}
		rules = append(rules, r)
	}

	return &api.AgentToolPolicy{
		Tools: &tools,
		Bash: &api.AgentBashPolicy{
			Rules:           &rules,
			DefaultBehavior: (*api.AgentBashPolicyDefaultBehavior)(&resolved.Bash.DefaultBehavior),
			DenyBehavior:    (*api.AgentBashPolicyDenyBehavior)(&resolved.Bash.DenyBehavior),
		},
	}
}

func toInternalToolPolicy(policy api.AgentToolPolicy) agent.ToolPolicyConfig {
	out := agent.ToolPolicyConfig{
		Tools: map[string]bool{},
	}

	if policy.Tools != nil {
		maps.Copy(out.Tools, *policy.Tools)
	}

	if policy.Bash == nil {
		return out
	}

	if policy.Bash.DefaultBehavior != nil {
		out.Bash.DefaultBehavior = agent.BashDefaultBehavior(*policy.Bash.DefaultBehavior)
	}
	if policy.Bash.DenyBehavior != nil {
		out.Bash.DenyBehavior = agent.BashDenyBehavior(*policy.Bash.DenyBehavior)
	}
	if policy.Bash.Rules != nil {
		out.Bash.Rules = make([]agent.BashRule, 0, len(*policy.Bash.Rules))
		for _, rule := range *policy.Bash.Rules {
			r := agent.BashRule{
				Pattern: rule.Pattern,
				Action:  agent.BashRuleAction(rule.Action),
			}
			if rule.Name != nil {
				r.Name = *rule.Name
			}
			if rule.Enabled != nil {
				r.Enabled = rule.Enabled
			}
			out.Bash.Rules = append(out.Bash.Rules, r)
		}
	}

	return out
}
