// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"net/url"
	"testing"

	api "github.com/dagucloud/dagu/api/v1"
	"github.com/stretchr/testify/require"
)

func TestParseWorkspaceSelection(t *testing.T) {
	t.Run("defaults to all", func(t *testing.T) {
		selection, err := parseWorkspaceSelection(nil)
		require.NoError(t, err)
		require.Equal(t, workspaceSelectionAll, selection.mode)
		require.Empty(t, selection.workspace)
		require.False(t, selection.explicit)
	})

	t.Run("accepts named workspace", func(t *testing.T) {
		workspace := api.Workspace("ops")
		selection, err := parseWorkspaceSelection(&workspace)
		require.NoError(t, err)
		require.Equal(t, workspaceSelectionNamed, selection.mode)
		require.Equal(t, "ops", selection.workspace)
		require.True(t, selection.explicit)
	})

	t.Run("accepts all", func(t *testing.T) {
		workspace := api.Workspace("all")
		selection, err := parseWorkspaceSelection(&workspace)
		require.NoError(t, err)
		require.Equal(t, workspaceSelectionAll, selection.mode)
		require.Empty(t, selection.workspace)
		require.True(t, selection.explicit)
	})

	t.Run("accepts default", func(t *testing.T) {
		workspace := api.Workspace("default")
		selection, err := parseWorkspaceSelection(&workspace)
		require.NoError(t, err)
		require.Equal(t, workspaceSelectionDefault, selection.mode)
		require.Empty(t, selection.workspace)
		require.True(t, selection.explicit)
	})
}

func TestWorkspaceParamFromValuesRejectsExplicitEmptyWorkspace(t *testing.T) {
	params := url.Values{"workspace": []string{""}}

	workspace := workspaceParamFromValues(params)
	require.NotNil(t, workspace)
	require.Empty(t, *workspace)

	_, err := parseWorkspaceSelection(workspace)
	require.Error(t, err)
}

func TestWorkspaceParamFromValuesPreservesExplicitEmptyWorkspace(t *testing.T) {
	params := url.Values{"workspace": []string{""}}

	workspace := workspaceParamFromValues(params)
	require.NotNil(t, workspace)
	require.Empty(t, *workspace)
}
