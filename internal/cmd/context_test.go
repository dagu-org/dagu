// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/cmn/config"
	_ "github.com/dagucloud/dagu/internal/llm/allproviders"
	"github.com/dagucloud/dagu/internal/persis/fileagentconfig"
	"github.com/dagucloud/dagu/internal/persis/fileagentmodel"
	"github.com/dagucloud/dagu/internal/persis/fileagentoauth"
	"github.com/dagucloud/dagu/internal/persis/fileagentskill"
	"github.com/dagucloud/dagu/internal/persis/fileagentsoul"
	"github.com/dagucloud/dagu/internal/persis/filememory"
	"github.com/dagucloud/dagu/internal/persis/filesession"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_StringParam(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		flagValue string
		expected  string
		expectErr bool
	}{
		{
			name:      "StringParamWithoutQuotes",
			flagName:  "test-param",
			flagValue: "hello",
			expected:  "hello",
			expectErr: false,
		},
		{
			name:      "StringParamWithDoubleQuotes",
			flagName:  "test-param",
			flagValue: `"world"`,
			expected:  "world",
			expectErr: false,
		},
		{
			name:      "EmptyStringParam",
			flagName:  "test-param",
			flagValue: `""`,
			expected:  "",
			expectErr: false,
		},
		{
			name:      "StringParamWithEscapedDoubleQuotes",
			flagName:  "test-param",
			flagValue: `"{\"key\":\"value with \\\"quotes\\\"\"}"`, // This is the string literal `{"key":"value with \"quotes\""}`
			expected:  `{"key":"value with \"quotes\""}`,
			expectErr: false,
		},
		{
			name:      "JSONStringParam",
			flagName:  "test-param",
			flagValue: `"{ \"name\": \"test\", \"value\": 123 }"`, // This is the string literal `{ "name": "test", "value": 123 }`
			expected:  `{ "name": "test", "value": 123 }`,
			expectErr: false,
		},
		{
			name:      "FlagNotFound",
			flagName:  "non-existent-param",
			flagValue: "", // Value doesn't matter if flag not found
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use: "test",
			}
			if tt.flagName != "non-existent-param" { // Only add flag if it's expected to exist
				cmd.Flags().String(tt.flagName, "", "test flag")
				_ = cmd.Flags().Set(tt.flagName, tt.flagValue)
			}

			ctx := &Context{
				Command: cmd,
			}

			val, err := ctx.StringParam(tt.flagName)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error but got: %v", err)
				}
				if val != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, val)
				}
			}
		})
	}
}

func TestContext_newSchedulerAgentAPI_WiresOAuthManagerForCodexDefaultModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctxBase, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := &config.Config{
		Paths: config.PathsConfig{
			DAGsDir:        filepath.Join(root, "dags"),
			DocsDir:        filepath.Join(root, "docs"),
			LogDir:         filepath.Join(root, "logs"),
			DataDir:        filepath.Join(root, "data"),
			SessionsDir:    filepath.Join(root, "sessions"),
			ConfigFileUsed: filepath.Join(root, "config.yaml"),
			BaseConfig:     filepath.Join(root, "base.yaml"),
		},
		Server: config.Server{Session: config.SessionConfig{MaxPerUser: 10}},
	}
	cmdCtx := &Context{
		Context: ctxBase,
		Config:  cfg,
	}

	configStore, err := fileagentconfig.New(cfg.Paths.DataDir)
	require.NoError(t, err)
	agentCfg := agent.DefaultConfig()
	agentCfg.DefaultModelID = "codex-default"
	require.NoError(t, configStore.Save(ctxBase, agentCfg))

	modelStore, err := fileagentmodel.New(filepath.Join(cfg.Paths.DataDir, "agent", "models"))
	require.NoError(t, err)
	require.NoError(t, modelStore.Create(ctxBase, &agent.ModelConfig{
		ID:       "codex-default",
		Name:     "Codex Default",
		Provider: "openai-codex",
		Model:    "gpt-5-3-codex",
	}))

	skillStore, err := fileagentskill.New(filepath.Join(cfg.Paths.DAGsDir, "skills"))
	require.NoError(t, err)
	soulStore, err := fileagentsoul.New(ctxBase, filepath.Join(cfg.Paths.DAGsDir, "souls"))
	require.NoError(t, err)
	sessionStore, err := filesession.New(cfg.Paths.SessionsDir, filesession.WithMaxPerUser(cfg.Server.Session.MaxPerUser))
	require.NoError(t, err)
	memoryStore, err := filememory.New(cfg.Paths.DAGsDir)
	require.NoError(t, err)
	oauthManager, err := fileagentoauth.NewManager(cfg.Paths.DataDir)
	require.NoError(t, err)

	api := cmdCtx.newSchedulerAgentAPI(nil, configStore, modelStore, skillStore, soulStore, sessionStore, memoryStore, oauthManager)

	sessionID, err := api.CreateEmptySessionWithRuntime(ctxBase, agent.UserIdentity{
		UserID:   "admin",
		Username: "admin",
		Role:     auth.RoleAdmin,
	}, "", false, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
}
