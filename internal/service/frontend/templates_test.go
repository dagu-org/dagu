// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	apiv1 "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/cmn/config"
	workspacepkg "github.com/dagucloud/dagu/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubWorkspaceStore struct {
	items []*workspacepkg.Workspace
}

func (s stubWorkspaceStore) Create(context.Context, *workspacepkg.Workspace) error {
	return nil
}

func (s stubWorkspaceStore) GetByID(context.Context, string) (*workspacepkg.Workspace, error) {
	return nil, nil
}

func (s stubWorkspaceStore) GetByName(context.Context, string) (*workspacepkg.Workspace, error) {
	return nil, nil
}

func (s stubWorkspaceStore) List(context.Context) ([]*workspacepkg.Workspace, error) {
	return s.items, nil
}

func (s stubWorkspaceStore) Update(context.Context, *workspacepkg.Workspace) error {
	return nil
}

func (s stubWorkspaceStore) Delete(context.Context, string) error {
	return nil
}

func resetAssetVersionCache() {
	assetVersion = ""
	assetVersionOnce = sync.Once{}
}

func TestFormatAssetVersionUsesBundleHashForDevBuilds(t *testing.T) {
	bundle := []byte("bundle")
	sum := sha256.Sum256(bundle)
	want := "0.0.0-" + hex.EncodeToString(sum[:8])

	assert.Equal(t, want, formatAssetVersion("0.0.0", bundle))
}

func TestFormatAssetVersionSupportsEmptyVersion(t *testing.T) {
	bundle := []byte("bundle")
	sum := sha256.Sum256(bundle)
	want := hex.EncodeToString(sum[:8])

	assert.Equal(t, want, formatAssetVersion("", bundle))
}

func TestCurrentAssetVersionUsesReleaseVersionWhenSet(t *testing.T) {
	originalVersion := config.Version
	t.Cleanup(func() {
		config.Version = originalVersion
		resetAssetVersionCache()
	})

	config.Version = "1.2.3"
	resetAssetVersionCache()

	assert.Equal(t, "1.2.3", currentAssetVersion())
}

func TestDefaultFunctionsExposeInitialWorkspacesJSON(t *testing.T) {
	createdAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.March, 2, 10, 0, 0, 0, time.UTC)
	funcs := defaultFunctions(&funcsConfig{
		WorkspaceStore: stubWorkspaceStore{
			items: []*workspacepkg.Workspace{
				{
					ID:          "ws-1",
					Name:        "ops",
					Description: "Operations",
					CreatedAt:   createdAt,
					UpdatedAt:   updatedAt,
				},
			},
		},
	})

	initialWorkspacesJSON, ok := funcs["initialWorkspacesJSON"].(func() string)
	require.True(t, ok)

	var workspaces []apiv1.WorkspaceResponse
	err := json.Unmarshal([]byte(initialWorkspacesJSON()), &workspaces)
	require.NoError(t, err)
	require.Len(t, workspaces, 1)
	assert.Equal(t, "ws-1", workspaces[0].Id)
	assert.Equal(t, "ops", workspaces[0].Name)
	require.NotNil(t, workspaces[0].Description)
	assert.Equal(t, "Operations", *workspaces[0].Description)
	require.NotNil(t, workspaces[0].CreatedAt)
	assert.True(t, workspaces[0].CreatedAt.Equal(createdAt))
	require.NotNil(t, workspaces[0].UpdatedAt)
	assert.True(t, workspaces[0].UpdatedAt.Equal(updatedAt))
}
