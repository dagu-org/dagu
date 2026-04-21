// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"net/url"
	"testing"

	api "github.com/dagucloud/dagu/api/v1"
	"github.com/stretchr/testify/require"
)

func TestParseWorkspaceScope(t *testing.T) {
	t.Run("defaults to all scope", func(t *testing.T) {
		selection, err := parseWorkspaceScope(nil, nil)
		require.NoError(t, err)
		require.Equal(t, api.WorkspaceScopeAll, selection.scope)
		require.Empty(t, selection.workspace)
		require.False(t, selection.explicit)
	})

	t.Run("keeps legacy workspace parameter as concrete workspace scope", func(t *testing.T) {
		workspace := api.Workspace("ops")
		selection, err := parseWorkspaceScope(nil, &workspace)
		require.NoError(t, err)
		require.Equal(t, api.WorkspaceScopeWorkspace, selection.scope)
		require.Equal(t, "ops", selection.workspace)
		require.False(t, selection.explicit)
	})

	t.Run("accepts explicit default workspace scope", func(t *testing.T) {
		scope := api.WorkspaceScopeDefault
		selection, err := parseWorkspaceScope(&scope, nil)
		require.NoError(t, err)
		require.Equal(t, api.WorkspaceScopeDefault, selection.scope)
		require.Empty(t, selection.workspace)
		require.True(t, selection.explicit)
	})

	t.Run("rejects deprecated scope names", func(t *testing.T) {
		for _, value := range []string{"none", "accessible"} {
			scope := api.WorkspaceScope(value)
			_, err := parseWorkspaceScope(&scope, nil)
			require.Error(t, err)
		}
	})

	t.Run("requires workspace name for concrete workspace scope", func(t *testing.T) {
		scope := api.WorkspaceScopeWorkspace
		_, err := parseWorkspaceScope(&scope, nil)
		require.Error(t, err)
	})

	t.Run("rejects workspace name on aggregate scope", func(t *testing.T) {
		scope := api.WorkspaceScopeAll
		workspace := api.Workspace("ops")
		_, err := parseWorkspaceScope(&scope, &workspace)
		require.Error(t, err)
	})
}

func TestWorkspaceScopeParamsFromValuesPreservesExplicitEmptyScope(t *testing.T) {
	params := url.Values{"workspaceScope": []string{""}}

	scope, workspace := workspaceScopeParamsFromValues(params)
	require.NotNil(t, scope)
	require.Empty(t, *scope)
	require.Nil(t, workspace)

	_, err := parseWorkspaceScope(scope, workspace)
	require.Error(t, err)
}

func TestWorkspaceScopeParamsFromValuesRejectsExplicitEmptyWorkspace(t *testing.T) {
	params := url.Values{"workspace": []string{""}}

	scope, workspace := workspaceScopeParamsFromValues(params)
	require.Nil(t, scope)
	require.NotNil(t, workspace)
	require.Empty(t, *workspace)

	_, err := parseWorkspaceScope(scope, workspace)
	require.Error(t, err)
}

func TestWorkspaceParamFromValuesPreservesExplicitEmptyWorkspace(t *testing.T) {
	params := url.Values{"workspace": []string{""}}

	workspace := workspaceParamFromValues(params)
	require.NotNil(t, workspace)
	require.Empty(t, *workspace)
}
