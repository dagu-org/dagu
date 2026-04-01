// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

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

	providerDiscoveryTimeout = 5 * time.Second
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

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load agent config", tag.Error(err))
		return nil, ErrFailedToLoadAgentConfig
	}
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
		return nil, ErrInvalidRequestBody
	}

	body := request.Body

	if err := validateProvider(string(body.Provider)); err != nil {
		return nil, err
	}

	// Generate or validate ID
	id := valueOf(body.Id)
	if id == "" {
		existingIDs := a.collectModelIDs(ctx)
		id = agent.UniqueID(body.Name, existingIDs, "model")
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

	baseURL, err := normalizeProviderBaseURL(string(body.Provider), valueOf(body.BaseUrl))
	if err != nil {
		return nil, err
	}

	model := &agent.ModelConfig{
		ID:               id,
		Name:             body.Name,
		Provider:         string(body.Provider),
		Model:            body.Model,
		APIKey:           valueOf(body.ApiKey),
		BaseURL:          baseURL,
		ContextWindow:    valueOf(body.ContextWindow),
		MaxOutputTokens:  valueOf(body.MaxOutputTokens),
		InputCostPer1M:   valueOf(body.InputCostPer1M),
		OutputCostPer1M:  valueOf(body.OutputCostPer1M),
		SupportsThinking: valueOf(body.SupportsThinking),
		Description:      valueOf(body.Description),
	}

	if err := a.agentModelStore.Create(ctx, model); err != nil {
		if errors.Is(err, agent.ErrModelAlreadyExists) || errors.Is(err, agent.ErrModelNameAlreadyExists) {
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
		return nil, ErrInvalidRequestBody
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

	existing.BaseURL, err = normalizeProviderBaseURL(existing.Provider, existing.BaseURL)
	if err != nil {
		return nil, err
	}

	if err := a.agentModelStore.Update(ctx, existing); err != nil {
		if errors.Is(err, agent.ErrModelAlreadyExists) || errors.Is(err, agent.ErrModelNameAlreadyExists) {
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
		return nil, ErrInvalidRequestBody
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
		logger.Error(ctx, "Failed to load agent config", tag.Error(err))
		return nil, ErrFailedToLoadAgentConfig
	}

	cfg.DefaultModelID = modelID
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return nil, ErrFailedToSaveAgentConfig
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

// DiscoverAgentProviderMetadata performs best-effort metadata discovery for supported providers.
func (a *API) DiscoverAgentProviderMetadata(ctx context.Context, request api.DiscoverAgentProviderMetadataRequestObject) (api.DiscoverAgentProviderMetadataResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	body := request.Body
	provider := string(body.Provider)
	if err := validateProvider(provider); err != nil {
		return nil, err
	}

	baseURL, err := normalizeProviderBaseURL(provider, valueOf(body.BaseUrl))
	if err != nil {
		return discoveryFailureResponse(true, err.Error(), nil), nil
	}

	switch provider {
	case string(llm.ProviderLocal):
		return api.DiscoverAgentProviderMetadata200JSONResponse(discoverLocalProviderMetadata(ctx, baseURL, valueOf(body.ApiKey))), nil
	default:
		return api.DiscoverAgentProviderMetadata200JSONResponse{
			Success:   false,
			Supported: false,
			Models:    []api.DiscoveredProviderModel{},
			Warnings: []string{
				fmt.Sprintf("Provider metadata discovery is not supported for provider %q yet.", provider),
			},
		}, nil
	}
}

func validateProvider(provider string) error {
	if _, err := llm.ParseProviderType(provider); err != nil {
		return &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid provider '%s': valid options are anthropic, openai, gemini, openrouter, local, zai", provider),
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
		return ErrAgentConfigNotAvailable
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

func normalizeProviderBaseURL(provider, rawBaseURL string) (string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)
	if provider != string(llm.ProviderLocal) || baseURL == "" {
		return baseURL, nil
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid base URL for local provider: %q", rawBaseURL),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/v1"
	}

	return parsed.String(), nil
}

func discoveryFailureResponse(supported bool, message string, warnings []string) api.DiscoverAgentProviderMetadata200JSONResponse {
	return api.DiscoverAgentProviderMetadata200JSONResponse{
		Success:   false,
		Supported: supported,
		Models:    []api.DiscoveredProviderModel{},
		Warnings:  append([]string(nil), warnings...),
		Error:     ptrOf(message),
	}
}

func discoverLocalProviderMetadata(ctx context.Context, baseURL, apiKey string) api.DiscoverProviderMetadataResponse {
	if baseURL == "" {
		return api.DiscoverProviderMetadataResponse{
			Success:   false,
			Supported: true,
			Models:    []api.DiscoveredProviderModel{},
			Warnings:  []string{},
			Error:     ptrOf("base URL is required for local provider discovery"),
		}
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, providerDiscoveryTimeout)
	defer cancel()

	tagModels, err := fetchLocalOllamaTagModels(discoveryCtx, baseURL, apiKey)
	if err == nil {
		return api.DiscoverProviderMetadataResponse{
			Success:   true,
			Supported: true,
			Models:    tagModels,
			Warnings:  []string{},
		}
	}

	warnings := []string{
		fmt.Sprintf("Failed to load installed models from /api/tags: %v", err),
	}

	compatModels, compatErr := fetchLocalCompatibleModels(discoveryCtx, baseURL, apiKey)
	if compatErr == nil {
		return api.DiscoverProviderMetadataResponse{
			Success:   true,
			Supported: true,
			Models:    compatModels,
			Warnings:  warnings,
		}
	}

	return api.DiscoverProviderMetadataResponse{
		Success:   false,
		Supported: true,
		Models:    []api.DiscoveredProviderModel{},
		Warnings:  warnings,
		Error: ptrOf(
			fmt.Sprintf("Failed to discover local models via /api/tags and /v1/models: %v", compatErr),
		),
	}
}

func fetchLocalOllamaTagModels(ctx context.Context, baseURL, apiKey string) ([]api.DiscoveredProviderModel, error) {
	type ollamaTagModel struct {
		Model string `json:"model"`
		Name  string `json:"name"`
	}
	type ollamaTagResponse struct {
		Models []ollamaTagModel `json:"models"`
	}

	inventoryBaseURL := trimLocalV1Suffix(baseURL)
	tagsURL, err := joinProviderURLPath(inventoryBaseURL, "/api/tags")
	if err != nil {
		return nil, err
	}

	var payload ollamaTagResponse
	if err := getProviderJSON(ctx, tagsURL, apiKey, &payload); err != nil {
		return nil, err
	}

	models := make([]api.DiscoveredProviderModel, 0, len(payload.Models))
	for _, item := range payload.Models {
		id := strings.TrimSpace(item.Model)
		if id == "" {
			id = strings.TrimSpace(item.Name)
		}
		if id == "" {
			continue
		}

		model := api.DiscoveredProviderModel{Id: id}
		if displayName := strings.TrimSpace(item.Name); displayName != "" && displayName != id {
			model.DisplayName = &displayName
		}
		models = append(models, model)
	}

	sortDiscoveredProviderModels(models)
	return models, nil
}

func fetchLocalCompatibleModels(ctx context.Context, baseURL, apiKey string) ([]api.DiscoveredProviderModel, error) {
	type openAIModel struct {
		Id string `json:"id"`
	}
	type openAIModelListResponse struct {
		Data []openAIModel `json:"data"`
	}

	modelsURL, err := joinProviderURLPath(baseURL, "/models")
	if err != nil {
		return nil, err
	}

	var payload openAIModelListResponse
	if err := getProviderJSON(ctx, modelsURL, apiKey, &payload); err != nil {
		return nil, err
	}

	models := make([]api.DiscoveredProviderModel, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.Id)
		if id == "" {
			continue
		}
		models = append(models, api.DiscoveredProviderModel{Id: id})
	}

	sortDiscoveredProviderModels(models)
	return models, nil
}

func getProviderJSON(ctx context.Context, urlString, apiKey string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlString, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("received HTTP %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid response body: %w", err)
	}
	return nil
}

func joinProviderURLPath(baseURL, suffix string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base URL %q", baseURL)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/")
	parsed.Path = basePath + "/" + strings.TrimPrefix(suffix, "/")
	return parsed.String(), nil
}

func trimLocalV1Suffix(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	switch parsed.Path {
	case "/v1", "/v1/":
		parsed.Path = ""
		return parsed.String()
	default:
		return baseURL
	}
}

func sortDiscoveredProviderModels(models []api.DiscoveredProviderModel) {
	sort.Slice(models, func(i, j int) bool {
		leftLabel := valueOf(models[i].DisplayName)
		if leftLabel == "" {
			leftLabel = models[i].Id
		}
		rightLabel := valueOf(models[j].DisplayName)
		if rightLabel == "" {
			rightLabel = models[j].Id
		}
		if leftLabel == rightLabel {
			return models[i].Id < models[j].Id
		}
		return leftLabel < rightLabel
	})
}
