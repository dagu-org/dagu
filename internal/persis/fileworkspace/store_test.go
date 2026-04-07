// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileworkspace_test

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/internal/persis/fileworkspace"
	"github.com/dagucloud/dagu/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *fileworkspace.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := fileworkspace.New(dir)
	require.NoError(t, err)
	return store
}

func TestStore_CreateAndGetByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ws := workspace.NewWorkspace("test_ws", "A test workspace")
	require.NoError(t, store.Create(ctx, ws))

	got, err := store.GetByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Equal(t, ws.ID, got.ID)
	assert.Equal(t, ws.Name, got.Name)
	assert.Equal(t, ws.Description, got.Description)
}

func TestStore_GetByName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ws := workspace.NewWorkspace("named_ws", "")
	require.NoError(t, store.Create(ctx, ws))

	got, err := store.GetByName(ctx, "named_ws")
	require.NoError(t, err)
	assert.Equal(t, ws.ID, got.ID)
}

func TestStore_List(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, workspace.NewWorkspace("ws_1", "")))
	require.NoError(t, store.Create(ctx, workspace.NewWorkspace("ws_2", "")))

	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestStore_Update(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ws := workspace.NewWorkspace("original", "desc")
	require.NoError(t, store.Create(ctx, ws))

	ws.Name = "renamed"
	ws.Description = "updated"
	require.NoError(t, store.Update(ctx, ws))

	got, err := store.GetByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)
	assert.Equal(t, "updated", got.Description)

	// Old name should no longer resolve
	_, err = store.GetByName(ctx, "original")
	assert.ErrorIs(t, err, workspace.ErrWorkspaceNotFound)
}

func TestStore_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ws := workspace.NewWorkspace("to_delete", "")
	require.NoError(t, store.Create(ctx, ws))

	require.NoError(t, store.Delete(ctx, ws.ID))

	_, err := store.GetByID(ctx, ws.ID)
	assert.ErrorIs(t, err, workspace.ErrWorkspaceNotFound)
}

func TestStore_DuplicateName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, workspace.NewWorkspace("dup", "")))
	err := store.Create(ctx, workspace.NewWorkspace("dup", ""))
	assert.ErrorIs(t, err, workspace.ErrWorkspaceAlreadyExists)
}

func TestStore_CreateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Create(ctx, workspace.NewWorkspace("ops-team", ""))
	assert.ErrorIs(t, err, workspace.ErrInvalidWorkspaceName)
}

func TestStore_UpdateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ws := workspace.NewWorkspace("ops_team", "")
	require.NoError(t, store.Create(ctx, ws))

	ws.Name = "ops-team"
	err := store.Update(ctx, ws)
	assert.ErrorIs(t, err, workspace.ErrInvalidWorkspaceName)
}

func TestStore_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, workspace.ErrWorkspaceNotFound)

	_, err = store.GetByName(ctx, "nonexistent")
	assert.ErrorIs(t, err, workspace.ErrWorkspaceNotFound)

	err = store.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, workspace.ErrWorkspaceNotFound)
}
