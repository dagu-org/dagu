package api_test

import (
	"testing"

	apigen "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/runtime"
	apiV1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:fix inline
func strPtr(v string) *string { return &v }

func TestGetAgentConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns enabled and defaultModelId", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config.Enabled = true
		setup.configStore.config.DefaultModelID = "my-model"

		resp, err := setup.api.GetAgentConfig(adminCtx(), apigen.GetAgentConfigRequestObject{})
		require.NoError(t, err)

		getResp, ok := resp.(apigen.GetAgentConfig200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, getResp.Enabled)
		assert.True(t, *getResp.Enabled)
		require.NotNil(t, getResp.DefaultModelId)
		assert.Equal(t, "my-model", *getResp.DefaultModelId)
		require.NotNil(t, getResp.ToolPolicy)
		require.NotNil(t, getResp.ToolPolicy.Tools)
		require.Contains(t, *getResp.ToolPolicy.Tools, "bash")
	})

	t.Run("returns 403 when store not configured", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.GetAgentConfig(adminCtx(), apigen.GetAgentConfigRequestObject{})
		require.Error(t, err)
	})
}

func TestUpdateAgentConfig(t *testing.T) {
	t.Parallel()

	t.Run("partial update enabled only", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config.Enabled = true
		setup.configStore.config.DefaultModelID = "original"

		newEnabled := false
		resp, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				Enabled: &newEnabled,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentConfig200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updateResp.Enabled)
		assert.False(t, *updateResp.Enabled)
		// DefaultModelID should remain unchanged
		require.NotNil(t, updateResp.DefaultModelId)
		assert.Equal(t, "original", *updateResp.DefaultModelId)
	})

	t.Run("partial update defaultModelId only", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config.Enabled = true
		setup.configStore.config.DefaultModelID = "old-model"

		newDefault := "new-model"
		resp, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				DefaultModelId: &newDefault,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentConfig200JSONResponse)
		require.True(t, ok)
		// Enabled should remain unchanged
		require.NotNil(t, updateResp.Enabled)
		assert.True(t, *updateResp.Enabled)
		require.NotNil(t, updateResp.DefaultModelId)
		assert.Equal(t, "new-model", *updateResp.DefaultModelId)
	})

	t.Run("full update", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config.Enabled = true
		setup.configStore.config.DefaultModelID = "old"

		newEnabled := false
		newDefault := "new-default"
		_, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				Enabled:        &newEnabled,
				DefaultModelId: &newDefault,
			},
		})
		require.NoError(t, err)

		// Verify config store was updated
		assert.False(t, setup.configStore.config.Enabled)
		assert.Equal(t, "new-default", setup.configStore.config.DefaultModelID)
	})

	t.Run("updates tool policy", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		denyBehavior := apigen.AgentBashPolicyDenyBehaviorBlock
		defaultBehavior := apigen.AgentBashPolicyDefaultBehaviorAllow
		action := apigen.AgentBashRuleActionAllow
		enabled := true
		rules := []apigen.AgentBashRule{
			{
				Name:    strPtr("allow_git_status"),
				Pattern: "^git\\s+status$",
				Action:  action,
				Enabled: &enabled,
			},
		}

		resp, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				ToolPolicy: &apigen.AgentToolPolicy{
					Tools: &map[string]bool{"bash": true, "patch": true},
					Bash: &apigen.AgentBashPolicy{
						Rules:           &rules,
						DefaultBehavior: &defaultBehavior,
						DenyBehavior:    &denyBehavior,
					},
				},
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentConfig200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updateResp.ToolPolicy)
		require.NotNil(t, updateResp.ToolPolicy.Bash)
		require.NotNil(t, updateResp.ToolPolicy.Bash.Rules)
		require.Len(t, *updateResp.ToolPolicy.Bash.Rules, 1)
		require.NotNil(t, updateResp.ToolPolicy.Bash.DefaultBehavior)
		require.Equal(t, defaultBehavior, *updateResp.ToolPolicy.Bash.DefaultBehavior)
		require.NotNil(t, updateResp.ToolPolicy.Bash.DenyBehavior)
		require.Equal(t, denyBehavior, *updateResp.ToolPolicy.Bash.DenyBehavior)
	})

	t.Run("invalid tool policy returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		action := apigen.AgentBashRuleActionAllow
		rules := []apigen.AgentBashRule{
			{
				Pattern: "([",
				Action:  action,
			},
		}

		_, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				ToolPolicy: &apigen.AgentToolPolicy{
					Bash: &apigen.AgentBashPolicy{
						Rules: &rules,
					},
				},
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)

		_, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})

	t.Run("returns 403 when store not configured", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		newEnabled := false
		_, err := a.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				Enabled: &newEnabled,
			},
		})
		require.Error(t, err)
	})
}

func TestUpdateAgentConfig_PersistsChanges(t *testing.T) {
	t.Parallel()

	t.Run("persists changes correctly", func(t *testing.T) {
		t.Parallel()

		setup := newAgentTestSetup(t)
		setup.configStore.config = &agent.Config{
			Enabled:        true,
			DefaultModelID: "model-1",
		}

		newEnabled := false
		newDefault := "model-2"
		_, err := setup.api.UpdateAgentConfig(adminCtx(), apigen.UpdateAgentConfigRequestObject{
			Body: &apigen.UpdateAgentConfigRequest{
				Enabled:        &newEnabled,
				DefaultModelId: &newDefault,
			},
		})
		require.NoError(t, err)

		// Verify underlying store was updated
		assert.False(t, setup.configStore.config.Enabled)
		assert.Equal(t, "model-2", setup.configStore.config.DefaultModelID)
	})
}
