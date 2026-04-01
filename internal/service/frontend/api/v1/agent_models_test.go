// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	apigen "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/runtime"
	apiV1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentTestSetup contains common test infrastructure for agent API tests.
type agentTestSetup struct {
	api         *apiV1.API
	modelStore  *mockAgentModelStore
	configStore *mockAgentConfigStore
}

func newAgentTestSetup(t *testing.T) *agentTestSetup {
	t.Helper()

	ms := &mockAgentModelStore{models: make(map[string]*agent.ModelConfig), byName: make(map[string]string)}
	cs := &mockAgentConfigStore{config: agent.DefaultConfig()}

	cfg := &config.Config{}
	a := apiV1.New(
		nil, nil, nil, nil, runtime.Manager{},
		cfg, nil, nil,
		prometheus.NewRegistry(),
		nil,
		apiV1.WithAgentModelStore(ms),
		apiV1.WithAgentConfigStore(cs),
	)

	return &agentTestSetup{
		api:         a,
		modelStore:  ms,
		configStore: cs,
	}
}

func adminCtx() context.Context {
	return auth.WithUser(context.Background(), &auth.User{
		ID:       "admin-1",
		Username: "admin",
		Role:     auth.RoleAdmin,
	})
}

type mockAgentModelStore struct {
	models map[string]*agent.ModelConfig
	byName map[string]string // name -> ID
}

func (m *mockAgentModelStore) Create(_ context.Context, model *agent.ModelConfig) error {
	if _, exists := m.models[model.ID]; exists {
		return agent.ErrModelAlreadyExists
	}
	if takenByID, exists := m.byName[model.Name]; exists && takenByID != model.ID {
		return agent.ErrModelNameAlreadyExists
	}
	m.models[model.ID] = model
	m.byName[model.Name] = model.ID
	return nil
}

func (m *mockAgentModelStore) GetByID(_ context.Context, id string) (*agent.ModelConfig, error) {
	model, ok := m.models[id]
	if !ok {
		return nil, agent.ErrModelNotFound
	}
	return model, nil
}

func (m *mockAgentModelStore) List(_ context.Context) ([]*agent.ModelConfig, error) {
	var result []*agent.ModelConfig
	for _, model := range m.models {
		result = append(result, model)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (m *mockAgentModelStore) Update(_ context.Context, model *agent.ModelConfig) error {
	existing, ok := m.models[model.ID]
	if !ok {
		return agent.ErrModelNotFound
	}
	if takenByID, exists := m.byName[model.Name]; exists && takenByID != model.ID {
		return agent.ErrModelNameAlreadyExists
	}
	delete(m.byName, existing.Name)
	m.models[model.ID] = model
	m.byName[model.Name] = model.ID
	return nil
}

func (m *mockAgentModelStore) Delete(_ context.Context, id string) error {
	model, ok := m.models[id]
	if !ok {
		return agent.ErrModelNotFound
	}
	delete(m.byName, model.Name)
	delete(m.models, id)
	return nil
}

// addModel is a test helper that populates both indexes.
func (m *mockAgentModelStore) addModel(model *agent.ModelConfig) {
	m.models[model.ID] = model
	m.byName[model.Name] = model.ID
}

var _ agent.ModelStore = (*mockAgentModelStore)(nil)

type mockAgentConfigStore struct {
	config *agent.Config
}

func (m *mockAgentConfigStore) Load(_ context.Context) (*agent.Config, error) {
	return m.config, nil
}

func (m *mockAgentConfigStore) Save(_ context.Context, cfg *agent.Config) error {
	m.config = cfg
	return nil
}

func (m *mockAgentConfigStore) IsEnabled(_ context.Context) bool {
	return m.config.Enabled
}

var _ agent.ConfigStore = (*mockAgentConfigStore)(nil)

func TestListAgentModels(t *testing.T) {
	t.Parallel()

	t.Run("returns models and defaultModelId", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Model 1", Provider: "openai", Model: "gpt-4",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-2", Name: "Model 2", Provider: "anthropic", Model: "claude-sonnet-4-5",
		})
		setup.configStore.config.DefaultModelID = "model-1"

		resp, err := setup.api.ListAgentModels(adminCtx(), apigen.ListAgentModelsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentModels200JSONResponse)
		require.True(t, ok)
		assert.Len(t, listResp.Models, 2)
		require.NotNil(t, listResp.DefaultModelId)
		assert.Equal(t, "model-1", *listResp.DefaultModelId)
	})

	t.Run("returns empty list when no models", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.ListAgentModels(adminCtx(), apigen.ListAgentModelsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentModels200JSONResponse)
		require.True(t, ok)
		assert.Empty(t, listResp.Models)
	})

	t.Run("returns 403 when store not configured", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.ListAgentModels(adminCtx(), apigen.ListAgentModelsRequestObject{})
		require.Error(t, err)
	})
}

func TestCreateAgentModel(t *testing.T) {
	t.Parallel()

	t.Run("valid create returns 201", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Test Model",
				Provider: "openai",
				Model:    "gpt-4",
				ApiKey:   new("sk-test"),
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentModel201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Test Model", createResp.Name)
		assert.Equal(t, apigen.ModelConfigResponseProvider("openai"), createResp.Provider)
		assert.NotEmpty(t, createResp.Id)
	})

	t.Run("invalid provider returns 400", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Bad Provider",
				Provider: "invalid-provider",
				Model:    "gpt-4",
			},
		})
		require.Error(t, err)
	})

	t.Run("invalid model ID returns 400", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Id:       new("INVALID ID"),
				Name:     "Test",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.Error(t, err)
	})

	t.Run("empty name returns 400", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.Error(t, err)
	})

	t.Run("empty model returns 400", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Test",
				Provider: "openai",
				Model:    "",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate ID returns 409", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "test-model", Name: "Existing", Provider: "openai", Model: "gpt-4",
		})

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Id:       new("test-model"),
				Name:     "Duplicate",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate name returns 409", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "existing-model", Name: "Taken Name", Provider: "openai", Model: "gpt-4",
		})

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Id:       new("different-id"),
				Name:     "Taken Name",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.Error(t, err)
	})

	t.Run("auto-sets default on first model", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		assert.Empty(t, setup.configStore.config.DefaultModelID)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "First Model",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.NoError(t, err)

		assert.NotEmpty(t, setup.configStore.config.DefaultModelID, "default should be auto-set for first model")
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})
}

func TestUpdateAgentModel(t *testing.T) {
	t.Parallel()

	t.Run("valid partial update returns 200", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Original", Provider: "openai", Model: "gpt-4",
		})

		newName := "Updated Name"
		resp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "model-1",
			Body: &apigen.UpdateModelConfigRequest{
				Name: &newName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Updated Name", updateResp.Name)
		assert.Equal(t, apigen.ModelConfigResponseProvider("openai"), updateResp.Provider) // unchanged
	})

	t.Run("model not found returns 404", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		newName := "Updated"
		_, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "nonexistent",
			Body: &apigen.UpdateModelConfigRequest{
				Name: &newName,
			},
		})
		require.Error(t, err)
	})

	t.Run("name conflict returns 409", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "First Model", Provider: "openai", Model: "gpt-4",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-2", Name: "Second Model", Provider: "openai", Model: "gpt-4",
		})

		conflictName := "First Model"
		_, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "model-2",
			Body: &apigen.UpdateModelConfigRequest{
				Name: &conflictName,
			},
		})
		require.Error(t, err)
	})

	t.Run("invalid provider on update returns 400", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Test", Provider: "openai", Model: "gpt-4",
		})

		badProvider := apigen.UpdateModelConfigRequestProvider("bad-provider")
		_, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "model-1",
			Body: &apigen.UpdateModelConfigRequest{
				Provider: &badProvider,
			},
		})
		require.Error(t, err)
	})
}

func TestDeleteAgentModel(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns 204", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Delete Me", Provider: "openai", Model: "gpt-4",
		})

		resp, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "model-1",
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.DeleteAgentModel204Response)
		assert.True(t, ok)

		_, exists := setup.modelStore.models["model-1"]
		assert.False(t, exists, "model should be deleted")
	})

	t.Run("not found returns 404", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "nonexistent",
		})
		require.Error(t, err)
	})

	t.Run("resets default if deleted model was default", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Default Model", Provider: "openai", Model: "gpt-4",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-2", Name: "Backup Model", Provider: "openai", Model: "gpt-3.5",
		})
		setup.configStore.config.DefaultModelID = "model-1"

		_, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "model-1",
		})
		require.NoError(t, err)

		// Default should be reset to remaining model
		assert.Equal(t, "model-2", setup.configStore.config.DefaultModelID)
	})
}

func TestSetDefaultAgentModel(t *testing.T) {
	t.Parallel()

	t.Run("valid set returns 200", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-1", Name: "Test", Provider: "openai", Model: "gpt-4",
		})

		resp, err := setup.api.SetDefaultAgentModel(adminCtx(), apigen.SetDefaultAgentModelRequestObject{
			Body: &apigen.SetDefaultAgentModelJSONRequestBody{
				ModelId: "model-1",
			},
		})
		require.NoError(t, err)

		setResp, ok := resp.(apigen.SetDefaultAgentModel200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, setResp.DefaultModelId)
		assert.Equal(t, "model-1", *setResp.DefaultModelId)
	})

	t.Run("model not found returns 404", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.SetDefaultAgentModel(adminCtx(), apigen.SetDefaultAgentModelRequestObject{
			Body: &apigen.SetDefaultAgentModelJSONRequestBody{
				ModelId: "nonexistent",
			},
		})
		require.Error(t, err)
	})
}

func TestListModelPresets(t *testing.T) {
	t.Parallel()

	t.Run("returns all presets", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.ListModelPresets(adminCtx(), apigen.ListModelPresetsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListModelPresets200JSONResponse)
		require.True(t, ok)
		assert.NotEmpty(t, listResp.Presets)

		// Verify presets have required fields
		for _, p := range listResp.Presets {
			assert.NotEmpty(t, p.Name)
			assert.NotEmpty(t, p.Model)
			assert.NotEmpty(t, p.Provider)
		}
	})
}

func TestApplyModelUpdates(t *testing.T) {
	t.Parallel()

	t.Run("nil fields not applied", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "m1", Name: "Original", Provider: "openai", Model: "gpt-4",
			APIKey: "key1", BaseURL: "http://example.com",
		})

		// Update with all nil fields
		resp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "m1",
			Body:    &apigen.UpdateModelConfigRequest{},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
		assert.Equal(t, apigen.ModelConfigResponseProvider("openai"), updateResp.Provider)
		assert.Equal(t, "gpt-4", updateResp.Model)
	})

	t.Run("empty-string name not applied", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "m1", Name: "Original", Provider: "openai", Model: "gpt-4",
		})

		emptyName := "  "
		resp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "m1",
			Body: &apigen.UpdateModelConfigRequest{
				Name: &emptyName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
	})

	t.Run("empty-string model not applied", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "m1", Name: "Test", Provider: "openai", Model: "gpt-4",
		})

		emptyModel := "  "
		resp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "m1",
			Body: &apigen.UpdateModelConfigRequest{
				Model: &emptyModel,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "gpt-4", updateResp.Model)
	})

	t.Run("all fields applied when set", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "m1", Name: "Original", Provider: "openai", Model: "gpt-4",
		})

		newName := "Updated"
		newModel := "gpt-5"
		newProvider := apigen.UpdateModelConfigRequestProvider("anthropic")
		newKey := "new-key"
		newURL := "https://new.example.com"
		newCtx := 100000
		newMax := 50000
		newInput := 5.0
		newOutput := 25.0
		newThinking := true
		newDesc := "Updated description"

		resp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "m1",
			Body: &apigen.UpdateModelConfigRequest{
				Name:             &newName,
				Model:            &newModel,
				Provider:         &newProvider,
				ApiKey:           &newKey,
				BaseUrl:          &newURL,
				ContextWindow:    &newCtx,
				MaxOutputTokens:  &newMax,
				InputCostPer1M:   &newInput,
				OutputCostPer1M:  &newOutput,
				SupportsThinking: &newThinking,
				Description:      &newDesc,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Updated", updateResp.Name)
		assert.Equal(t, "gpt-5", updateResp.Model)
		assert.Equal(t, apigen.ModelConfigResponseProvider("anthropic"), updateResp.Provider)
	})
}

func TestAutoSetDefaultModel(t *testing.T) {
	t.Parallel()

	t.Run("sets default when none exists", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		assert.Empty(t, setup.configStore.config.DefaultModelID)

		// Create first model
		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "First",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.NoError(t, err)

		assert.NotEmpty(t, setup.configStore.config.DefaultModelID)
	})

	t.Run("does not override existing default", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config.DefaultModelID = "existing-default"

		_, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Another",
				Provider: "openai",
				Model:    "gpt-4",
			},
		})
		require.NoError(t, err)

		assert.Equal(t, "existing-default", setup.configStore.config.DefaultModelID)
	})
}

func TestResetDefaultIfNeeded(t *testing.T) {
	t.Parallel()

	t.Run("resets to first remaining model", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-a", Name: "A", Provider: "openai", Model: "gpt-4",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-b", Name: "B", Provider: "openai", Model: "gpt-3.5",
		})
		setup.configStore.config.DefaultModelID = "model-a"

		_, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "model-a",
		})
		require.NoError(t, err)

		assert.Equal(t, "model-b", setup.configStore.config.DefaultModelID)
	})

	t.Run("clears default when no models left", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "only-model", Name: "Only", Provider: "openai", Model: "gpt-4",
		})
		setup.configStore.config.DefaultModelID = "only-model"

		_, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "only-model",
		})
		require.NoError(t, err)

		assert.Empty(t, setup.configStore.config.DefaultModelID)
	})

	t.Run("no-op when deleted model was not default", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-a", Name: "A", Provider: "openai", Model: "gpt-4",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "model-b", Name: "B", Provider: "openai", Model: "gpt-3.5",
		})
		setup.configStore.config.DefaultModelID = "model-b"

		_, err := setup.api.DeleteAgentModel(adminCtx(), apigen.DeleteAgentModelRequestObject{
			ModelId: "model-a",
		})
		require.NoError(t, err)

		assert.Equal(t, "model-b", setup.configStore.config.DefaultModelID, "default should not change")
	})
}

func TestDiscoverAgentProviderMetadata(t *testing.T) {
	t.Parallel()

	t.Run("discovers local models from ollama tags and forwards auth", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		var mu sync.Mutex
		var paths []string
		authHeader := ""

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			paths = append(paths, r.URL.Path)
			authHeader = r.Header.Get("Authorization")
			mu.Unlock()

			require.Equal(t, "/api/tags", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{
					{"name": "llama3.2:latest", "model": "llama3.2:latest"},
					{"name": "gemma3:latest", "model": "gemma3:latest"},
				},
			}))
		}))
		defer server.Close()

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new(server.URL),
				ApiKey:   new("local-token"),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.True(t, discoveryResp.Success)
		assert.True(t, discoveryResp.Supported)
		require.Len(t, discoveryResp.Models, 2)
		assert.Equal(t, "gemma3:latest", discoveryResp.Models[0].Id)
		assert.Equal(t, "llama3.2:latest", discoveryResp.Models[1].Id)
		assert.Empty(t, discoveryResp.Warnings)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{"/api/tags"}, paths)
		assert.Equal(t, "Bearer local-token", authHeader)
	})

	t.Run("strips v1 from local discovery tags endpoint", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		var requestedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{
					{"name": "llama3.2:latest", "model": "llama3.2:latest"},
				},
			}))
		}))
		defer server.Close()

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new(server.URL + "/v1"),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.True(t, discoveryResp.Success)
		assert.Equal(t, "/api/tags", requestedPath)
	})

	t.Run("falls back to v1 models when tags endpoint fails", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		var paths []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/api/tags":
				http.Error(w, "not found", http.StatusNotFound)
			case "/v1/models":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]string{
						{"id": "llama3.2"},
						{"id": "gemma3"},
					},
				}))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new(server.URL),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.True(t, discoveryResp.Success)
		assert.True(t, discoveryResp.Supported)
		require.Len(t, discoveryResp.Models, 2)
		assert.Equal(t, []string{"/api/tags", "/v1/models"}, paths)
		require.Len(t, discoveryResp.Warnings, 1)
		assert.Contains(t, discoveryResp.Warnings[0], "/api/tags")
	})

	t.Run("returns failure for invalid base url", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new("://bad url"),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.False(t, discoveryResp.Success)
		assert.True(t, discoveryResp.Supported)
		require.NotNil(t, discoveryResp.Error)
		assert.Contains(t, *discoveryResp.Error, "invalid base URL")
	})

	t.Run("returns failure when provider cannot be reached", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new("http://127.0.0.1:1"),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.False(t, discoveryResp.Success)
		assert.True(t, discoveryResp.Supported)
		assert.Empty(t, discoveryResp.Models)
		require.NotNil(t, discoveryResp.Error)
		assert.Contains(t, *discoveryResp.Error, "Failed to discover local models")
		require.Len(t, discoveryResp.Warnings, 1)
	})

	t.Run("returns failure for malformed discovery responses", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{not-json"))
		}))
		defer server.Close()

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderLocal,
				BaseUrl:  new(server.URL),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.False(t, discoveryResp.Success)
		assert.True(t, discoveryResp.Supported)
		require.NotNil(t, discoveryResp.Error)
		assert.Contains(t, *discoveryResp.Error, "invalid response body")
	})

	t.Run("returns unsupported shape for providers without adapters", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		resp, err := setup.api.DiscoverAgentProviderMetadata(adminCtx(), apigen.DiscoverAgentProviderMetadataRequestObject{
			Body: &apigen.DiscoverProviderMetadataRequest{
				Provider: apigen.DiscoverProviderMetadataRequestProviderOpenai,
				BaseUrl:  new("https://api.openai.com/v1"),
			},
		})
		require.NoError(t, err)

		discoveryResp, ok := resp.(apigen.DiscoverAgentProviderMetadata200JSONResponse)
		require.True(t, ok)
		assert.False(t, discoveryResp.Success)
		assert.False(t, discoveryResp.Supported)
		assert.Empty(t, discoveryResp.Models)
		require.Len(t, discoveryResp.Warnings, 1)
		assert.Contains(t, discoveryResp.Warnings[0], "not supported")
		assert.Nil(t, discoveryResp.Error)
	})
}

func TestAgentModelSaveNormalizesLocalBaseURL(t *testing.T) {
	t.Parallel()

	t.Run("create normalizes only local root urls", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		createLocalResp, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Local Root",
				Provider: apigen.CreateModelConfigRequestProviderLocal,
				Model:    "llama3.2",
				BaseUrl:  new("http://localhost:11434"),
			},
		})
		require.NoError(t, err)

		localModel, ok := createLocalResp.(apigen.CreateAgentModel201JSONResponse)
		require.True(t, ok)
		require.NotNil(t, localModel.BaseUrl)
		assert.Equal(t, "http://localhost:11434/v1", *localModel.BaseUrl)

		createPreservedResp, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "Local Custom Path",
				Provider: apigen.CreateModelConfigRequestProviderLocal,
				Model:    "llama3.2",
				BaseUrl:  new("http://localhost:11434/custom"),
			},
		})
		require.NoError(t, err)

		preservedModel, ok := createPreservedResp.(apigen.CreateAgentModel201JSONResponse)
		require.True(t, ok)
		require.NotNil(t, preservedModel.BaseUrl)
		assert.Equal(t, "http://localhost:11434/custom", *preservedModel.BaseUrl)

		createOpenAIResp, err := setup.api.CreateAgentModel(adminCtx(), apigen.CreateAgentModelRequestObject{
			Body: &apigen.CreateModelConfigRequest{
				Name:     "OpenAI Root",
				Provider: apigen.CreateModelConfigRequestProviderOpenai,
				Model:    "gpt-5",
				BaseUrl:  new("https://api.openai.com"),
			},
		})
		require.NoError(t, err)

		openAIModel, ok := createOpenAIResp.(apigen.CreateAgentModel201JSONResponse)
		require.True(t, ok)
		require.NotNil(t, openAIModel.BaseUrl)
		assert.Equal(t, "https://api.openai.com", *openAIModel.BaseUrl)
	})

	t.Run("update normalizes only local root urls", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "local-model", Name: "Local Model", Provider: "local", Model: "llama3.2",
		})
		setup.modelStore.addModel(&agent.ModelConfig{
			ID: "openai-model", Name: "OpenAI Model", Provider: "openai", Model: "gpt-5",
		})

		localRoot := "http://localhost:11434"
		localResp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "local-model",
			Body: &apigen.UpdateModelConfigRequest{
				BaseUrl: &localRoot,
			},
		})
		require.NoError(t, err)

		updatedLocal, ok := localResp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updatedLocal.BaseUrl)
		assert.Equal(t, "http://localhost:11434/v1", *updatedLocal.BaseUrl)

		openAIRoot := "https://api.openai.com"
		openAIResp, err := setup.api.UpdateAgentModel(adminCtx(), apigen.UpdateAgentModelRequestObject{
			ModelId: "openai-model",
			Body: &apigen.UpdateModelConfigRequest{
				BaseUrl: &openAIRoot,
			},
		})
		require.NoError(t, err)

		updatedOpenAI, ok := openAIResp.(apigen.UpdateAgentModel200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updatedOpenAI.BaseUrl)
		assert.Equal(t, "https://api.openai.com", *updatedOpenAI.BaseUrl)
	})
}
