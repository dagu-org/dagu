// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package workspace

import (
	"context"
	"errors"
)

// Common errors for workspace store operations.
var (
	ErrWorkspaceNotFound      = errors.New("workspace not found")
	ErrWorkspaceAlreadyExists = errors.New("workspace with this name already exists")
	ErrInvalidWorkspaceName   = errors.New("invalid workspace name")
	ErrInvalidWorkspaceID     = errors.New("invalid workspace ID")
)

// Store defines the interface for workspace persistence operations.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create stores a new workspace.
	// Returns ErrWorkspaceAlreadyExists if a workspace with the same name exists.
	Create(ctx context.Context, ws *Workspace) error

	// GetByID retrieves a workspace by its unique ID.
	// Returns ErrWorkspaceNotFound if the workspace does not exist.
	GetByID(ctx context.Context, id string) (*Workspace, error)

	// GetByName retrieves a workspace by its name.
	// Returns ErrWorkspaceNotFound if the workspace does not exist.
	GetByName(ctx context.Context, name string) (*Workspace, error)

	// List returns all workspaces in the store.
	List(ctx context.Context) ([]*Workspace, error)

	// Update modifies an existing workspace.
	// Returns ErrWorkspaceNotFound if the workspace does not exist.
	Update(ctx context.Context, ws *Workspace) error

	// Delete removes a workspace by its ID.
	// Returns ErrWorkspaceNotFound if the workspace does not exist.
	Delete(ctx context.Context, id string) error
}
