package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionModelCreate = "agent_model_create"
	auditActionModelUpdate = "agent_model_update"
	auditActionModelDelete = "agent_model_delete"
	auditActionModelSetDef = "agent_model_set_default"
)

var (
	errAgentModelStoreNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent model management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errModelNotFound = &Error{
		Code:       api.ErrorCodeNotFound,
		Message:    "Model not found",
		HTTPStatus: http.StatusNotFound,
	}

	errModelAlreadyExists = &Error{
		Code:       api.ErrorCodeAlreadyExists,
		Message:    "Model already exists",
		HTTPStatus: http.StatusConflict,
	}
)

// ListAgentModels returns all configured models. Requires admin role.
func (a *API) ListAgentModels(ctx context.Context, _ api.ListAgentModelsRequestObject) (api.ListAgentModelsResponseObject, error) {
	if err := a.requireAgentModelManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	models, err := a.agentModelStore.List(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list agent models", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to list models", HTTPStatus: http.StatusInternalServerError}
	}

	cfg, _ := a.agentConfigStore.Load(ctx)
	defaultModelID := ""
	if cfg != nil {
		defaultModelID = cfg.DefaultModelID
	}

	modelResponses := make([]api.ModelConfigResponse, 0, len(models))
	for _, m := range models {
		modelResponses = append(modelResponses, toModelConfigResponse(m))
	}
	resp := api.ListAgentModels200JSONResponse{
		Models:         modelResponses,
		DefaultModelId: ptrOf(defaultModelID),
	}

	return resp, nil
}

// CreateAgentModel creates a new model configuration. Requires admin role.
func (a *API) CreateAgentModel(ctx context.Context, request api.CreateAgentModelRequestObject) (api.CreateAgentModelResponseObject, error) {
	if err := a.requireAgentModelManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	body := request.Body

	if err := validateProvider(string(body.Provider)); err != nil {
		return nil, err
	}

	// Generate or validate ID
	id := valueOf(body.Id)
	if id == "" {
		existingIDs := a.collectModelIDs(ctx)
		id = agent.UniqueID(body.Name, existingIDs)
	}
	if err := agent.ValidateModelID(id); err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid model ID: %v", err),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	if strings.TrimSpace(body.Name) == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "name is required and cannot be empty",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if strings.TrimSpace(body.Model) == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "model is required and cannot be empty",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	model := &agent.ModelConfig{
		ID:               id,
		Name:             body.Name,
		Provider:         string(body.Provider),
		Model:            body.Model,
		APIKey:           valueOf(body.ApiKey),
		BaseURL:          valueOf(body.BaseUrl),
		ContextWindow:    valueOf(body.ContextWindow),
		MaxOutputTokens:  valueOf(body.MaxOutputTokens),
		InputCostPer1M:   valueOf(body.InputCostPer1M),
		OutputCostPer1M:  valueOf(body.OutputCostPer1M),
		SupportsThinking: valueOf(body.SupportsThinking),
		Description:      valueOf(body.Description),
	}

	if err := a.agentModelStore.Create(ctx, model); err != nil {
		if errors.Is(err, agent.ErrModelAlreadyExists) {
			return nil, errModelAlreadyExists
		}
		logger.Error(ctx, "Failed to create agent model", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to create model", HTTPStatus: http.StatusInternalServerError}
	}

	// If this is the first model, auto-set as default
	a.autoSetDefaultModel(ctx, id)

	a.logAudit(ctx, audit.CategoryAgent, auditActionModelCreate, map[string]any{
		"model_id": id,
		"name":     body.Name,
		"provider": string(body.Provider),
	})

	return api.CreateAgentModel201JSONResponse(toModelConfigResponse(model)), nil
}

// UpdateAgentModel updates an existing model configuration. Requires admin role.
func (a *API) UpdateAgentModel(ctx context.Context, request api.UpdateAgentModelRequestObject) (api.UpdateAgentModelResponseObject, error) {
	if err := a.requireAgentModelManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	body := request.Body

	// Validate provider before applying updates to avoid mutating with invalid data.
	if body.Provider != nil {
		if err := validateProvider(string(*body.Provider)); err != nil {
			return nil, err
		}
	}

	existing, err := a.agentModelStore.GetByID(ctx, request.ModelId)
	if err != nil {
		if errors.Is(err, agent.ErrModelNotFound) {
			return nil, errModelNotFound
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to get model", HTTPStatus: http.StatusInternalServerError}
	}

	applyModelUpdates(existing, body)

	if err := a.agentModelStore.Update(ctx, existing); err != nil {
		if errors.Is(err, agent.ErrModelAlreadyExists) {
			return nil, errModelAlreadyExists
		}
		logger.Error(ctx, "Failed to update agent model", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to update model", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionModelUpdate, map[string]any{
		"model_id": request.ModelId,
	})

	return api.UpdateAgentModel200JSONResponse(toModelConfigResponse(existing)), nil
}

// DeleteAgentModel removes a model configuration. Requires admin role.
func (a *API) DeleteAgentModel(ctx context.Context, request api.DeleteAgentModelRequestObject) (api.DeleteAgentModelResponseObject, error) {
	if err := a.requireAgentModelManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentModelStore.Delete(ctx, request.ModelId); err != nil {
		if errors.Is(err, agent.ErrModelNotFound) {
			return nil, errModelNotFound
		}
		logger.Error(ctx, "Failed to delete agent model", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to delete model", HTTPStatus: http.StatusInternalServerError}
	}

	// If deleted model was default, reset to first remaining
	a.resetDefaultIfNeeded(ctx, request.ModelId)

	a.logAudit(ctx, audit.CategoryAgent, auditActionModelDelete, map[string]any{
		"model_id": request.ModelId,
	})

	return api.DeleteAgentModel204Response{}, nil
}

// SetDefaultAgentModel sets the default model. Requires admin role.
func (a *API) SetDefaultAgentModel(ctx context.Context, request api.SetDefaultAgentModelRequestObject) (api.SetDefaultAgentModelResponseObject, error) {
	if err := a.requireAgentModelManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errInvalidRequestBody
	}

	modelID := request.Body.ModelId

	// Validate model exists
	if _, err := a.agentModelStore.GetByID(ctx, modelID); err != nil {
		if errors.Is(err, agent.ErrModelNotFound) {
			return nil, errModelNotFound
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to validate model", HTTPStatus: http.StatusInternalServerError}
	}

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		return nil, errFailedToLoadAgentConfig
	}

	cfg.DefaultModelID = modelID
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return nil, errFailedToSaveAgentConfig
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionModelSetDef, map[string]any{
		"model_id": modelID,
	})

	return api.SetDefaultAgentModel200JSONResponse{DefaultModelId: &modelID}, nil
}

// ListModelPresets returns available model presets. Requires admin role.
func (a *API) ListModelPresets(ctx context.Context, _ api.ListModelPresetsRequestObject) (api.ListModelPresetsResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	allPresets := agent.GetModelPresets()
	presets := make([]api.ModelPreset, 0, len(allPresets))
	for _, p := range allPresets {
		presets = append(presets, api.ModelPreset{
			Name:             p.Name,
			Provider:         api.ModelPresetProvider(p.Provider),
			Model:            p.Model,
			ContextWindow:    ptrOf(p.ContextWindow),
			MaxOutputTokens:  ptrOf(p.MaxOutputTokens),
			InputCostPer1M:   ptrOf(p.InputCostPer1M),
			OutputCostPer1M:  ptrOf(p.OutputCostPer1M),
			SupportsThinking: ptrOf(p.SupportsThinking),
			Description:      ptrOf(p.Description),
		})
	}

	return api.ListModelPresets200JSONResponse{Presets: presets}, nil
}

func validateProvider(provider string) error {
	if _, err := llm.ParseProviderType(provider); err != nil {
		return &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid provider '%s': valid options are anthropic, openai, gemini, openrouter, local", provider),
			HTTPStatus: http.StatusBadRequest,
		}
	}
	return nil
}

func (a *API) requireAgentModelManagement() error {
	if a.agentModelStore == nil {
		return errAgentModelStoreNotAvailable
	}
	if a.agentConfigStore == nil {
		return errAgentConfigNotAvailable
	}
	return nil
}

func toModelConfigResponse(m *agent.ModelConfig) api.ModelConfigResponse {
	return api.ModelConfigResponse{
		Id:               m.ID,
		Name:             m.Name,
		Provider:         api.ModelConfigResponseProvider(m.Provider),
		Model:            m.Model,
		ApiKeyConfigured: ptrOf(m.APIKey != ""),
		BaseUrl:          ptrOf(m.BaseURL),
		ContextWindow:    ptrOf(m.ContextWindow),
		MaxOutputTokens:  ptrOf(m.MaxOutputTokens),
		InputCostPer1M:   ptrOf(m.InputCostPer1M),
		OutputCostPer1M:  ptrOf(m.OutputCostPer1M),
		SupportsThinking: ptrOf(m.SupportsThinking),
		Description:      ptrOf(m.Description),
	}
}

func applyModelUpdates(model *agent.ModelConfig, update *api.UpdateModelConfigRequest) {
	if update.Name != nil && strings.TrimSpace(*update.Name) != "" {
		model.Name = *update.Name
	}
	if update.Provider != nil {
		model.Provider = string(*update.Provider)
	}
	if update.Model != nil && strings.TrimSpace(*update.Model) != "" {
		model.Model = *update.Model
	}
	if update.ApiKey != nil {
		model.APIKey = *update.ApiKey
	}
	if update.BaseUrl != nil {
		model.BaseURL = *update.BaseUrl
	}
	if update.ContextWindow != nil {
		model.ContextWindow = *update.ContextWindow
	}
	if update.MaxOutputTokens != nil {
		model.MaxOutputTokens = *update.MaxOutputTokens
	}
	if update.InputCostPer1M != nil {
		model.InputCostPer1M = *update.InputCostPer1M
	}
	if update.OutputCostPer1M != nil {
		model.OutputCostPer1M = *update.OutputCostPer1M
	}
	if update.SupportsThinking != nil {
		model.SupportsThinking = *update.SupportsThinking
	}
	if update.Description != nil {
		model.Description = *update.Description
	}
}

func (a *API) collectModelIDs(ctx context.Context) map[string]struct{} {
	models, err := a.agentModelStore.List(ctx)
	if err != nil {
		return make(map[string]struct{})
	}
	ids := make(map[string]struct{}, len(models))
	for _, m := range models {
		ids[m.ID] = struct{}{}
	}
	return ids
}

func (a *API) autoSetDefaultModel(ctx context.Context, modelID string) {
	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil || cfg.DefaultModelID != "" {
		return
	}
	cfg.DefaultModelID = modelID
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		logger.Error(ctx, "Failed to auto-set default model", tag.Error(err))
	}
}

func (a *API) resetDefaultIfNeeded(ctx context.Context, deletedModelID string) {
	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil || cfg.DefaultModelID != deletedModelID {
		return
	}

	models, err := a.agentModelStore.List(ctx)
	if err != nil || len(models) == 0 {
		cfg.DefaultModelID = ""
	} else {
		cfg.DefaultModelID = models[0].ID
	}
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		logger.Error(ctx, "Failed to reset default model after deletion", tag.Error(err))
	}
}
