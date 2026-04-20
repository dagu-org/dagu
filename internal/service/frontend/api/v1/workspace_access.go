// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
)

func toAPIWorkspaceAccess(access *auth.WorkspaceAccess) api.WorkspaceAccess {
	normalized := auth.NormalizeWorkspaceAccess(access)
	grants := make([]api.WorkspaceGrant, 0, len(normalized.Grants))
	for _, grant := range normalized.Grants {
		grants = append(grants, api.WorkspaceGrant{
			Workspace: grant.Workspace,
			Role:      api.UserRole(grant.Role),
		})
	}
	return api.WorkspaceAccess{
		All:    normalized.All,
		Grants: grants,
	}
}

func fromAPIWorkspaceAccess(access *api.WorkspaceAccess) (*auth.WorkspaceAccess, error) {
	if access == nil {
		return auth.AllWorkspaceAccess(), nil
	}

	grants := make([]auth.WorkspaceGrant, 0, len(access.Grants))
	for _, grant := range access.Grants {
		role, err := auth.ParseRole(string(grant.Role))
		if err != nil {
			return nil, err
		}
		grants = append(grants, auth.WorkspaceGrant{
			Workspace: grant.Workspace,
			Role:      role,
		})
	}

	return &auth.WorkspaceAccess{
		All:    access.All,
		Grants: grants,
	}, nil
}

func (a *API) parseAndValidateWorkspaceAccess(
	ctx context.Context,
	role auth.Role,
	access *api.WorkspaceAccess,
) (*auth.WorkspaceAccess, error) {
	parsed, err := fromAPIWorkspaceAccess(access)
	if err != nil {
		return nil, badWorkspaceAccessError("Invalid workspace role")
	}

	if !auth.NormalizeWorkspaceAccess(parsed).All {
		if a.workspaceStore == nil {
			return nil, workspaceStoreUnavailable()
		}
		workspaces, err := a.workspaceStore.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list workspaces for access validation: %w", err)
		}
		names := make(map[string]struct{}, len(workspaces))
		for _, ws := range workspaces {
			names[ws.Name] = struct{}{}
		}
		err = auth.ValidateWorkspaceAccess(role, parsed, func(name string) bool {
			_, ok := names[name]
			return ok
		})
	} else {
		err = auth.ValidateWorkspaceAccess(role, parsed, nil)
	}
	if err != nil {
		if errors.Is(err, auth.ErrInvalidWorkspaceAccess) {
			return nil, badWorkspaceAccessError(err.Error())
		}
		return nil, err
	}

	return auth.CloneWorkspaceAccess(parsed), nil
}

func badWorkspaceAccessError(message string) *Error {
	return &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

func workspaceResourceNotFound() *Error {
	return &Error{
		Code:       api.ErrorCodeNotFound,
		Message:    "Resource not found",
		HTTPStatus: http.StatusNotFound,
	}
}

func dagWorkspaceName(dag *core.DAG) string {
	if dag == nil {
		return ""
	}
	workspaceName, ok := exec.WorkspaceNameFromLabels(dag.Labels)
	if !ok {
		return ""
	}
	return workspaceName
}

func statusWorkspaceName(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	workspaceName, ok := exec.WorkspaceNameFromLabels(core.NewLabels(status.Labels))
	if !ok {
		return ""
	}
	return workspaceName
}

func workspaceNameFromLabelString(labels string) string {
	if strings.TrimSpace(labels) == "" {
		return ""
	}
	workspaceName, ok := exec.WorkspaceNameFromLabels(core.NewLabels(strings.Split(labels, ",")))
	if !ok {
		return ""
	}
	return workspaceName
}

func runtimeWorkspaceName(dag *core.DAG, labels string) string {
	if workspaceName := workspaceNameFromLabelString(labels); workspaceName != "" {
		return workspaceName
	}
	return dagWorkspaceName(dag)
}

func (a *API) workspaceFilterForContext(ctx context.Context) *exec.WorkspaceFilter {
	if a.authService == nil {
		return nil
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return &exec.WorkspaceFilter{Enabled: true}
	}
	access := auth.NormalizeWorkspaceAccess(user.WorkspaceAccess)
	if access.All {
		return nil
	}
	names := make([]string, 0, len(access.Grants))
	for _, grant := range access.Grants {
		names = append(names, grant.Workspace)
	}
	return &exec.WorkspaceFilter{
		Enabled:           true,
		Workspaces:        names,
		IncludeUnlabelled: true,
	}
}

func (a *API) effectiveRoleForWorkspace(ctx context.Context, workspaceName string) (auth.Role, bool, error) {
	if a.authService == nil {
		return auth.RoleAdmin, true, nil
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return auth.RoleNone, false, errAuthRequired
	}
	role, ok := auth.EffectiveRole(user.Role, user.WorkspaceAccess, workspaceName)
	return role, ok, nil
}

func (a *API) canAccessWorkspace(ctx context.Context, workspaceName string) bool {
	_, ok, err := a.effectiveRoleForWorkspace(ctx, workspaceName)
	return err == nil && ok
}

func (a *API) requireWorkspaceVisible(ctx context.Context, workspaceName string) error {
	_, ok, err := a.effectiveRoleForWorkspace(ctx, workspaceName)
	if err != nil {
		return err
	}
	if !ok {
		return workspaceResourceNotFound()
	}
	return nil
}

func (a *API) requireDAGWriteForWorkspace(ctx context.Context, workspaceName string) error {
	if !a.config.Server.Permissions[config.PermissionWriteDAGs] {
		return errPermissionDenied
	}
	role, ok, err := a.effectiveRoleForWorkspace(ctx, workspaceName)
	if err != nil {
		return err
	}
	if !ok || !role.CanWrite() {
		return errInsufficientPermissions
	}
	if a.dagWritesDisabled {
		return errDAGWritesDisabled
	}
	return nil
}

func (a *API) requireExecuteForWorkspace(ctx context.Context, workspaceName string) error {
	role, ok, err := a.effectiveRoleForWorkspace(ctx, workspaceName)
	if err != nil {
		return err
	}
	if !ok || !role.CanExecute() {
		return errInsufficientPermissions
	}
	return nil
}

func (a *API) workspaceNameForDAGRun(ctx context.Context, dagRun exec.DAGRunRef) (string, error) {
	attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return "", err
	}
	return workspaceNameForAttempt(ctx, attempt)
}

func workspaceNameForAttempt(ctx context.Context, attempt exec.DAGRunAttempt) (string, error) {
	status, err := attempt.ReadStatus(ctx)
	if err == nil && status != nil {
		return statusWorkspaceName(status), nil
	}
	dag, dagErr := attempt.ReadDAG(ctx)
	if dagErr != nil {
		if err != nil {
			return "", err
		}
		return "", dagErr
	}
	return dagWorkspaceName(dag), nil
}

func (a *API) requireDAGRunVisible(ctx context.Context, dagRun exec.DAGRunRef) error {
	if a.authService == nil {
		return nil
	}
	workspaceName, err := a.workspaceNameForDAGRun(ctx, dagRun)
	if err != nil {
		return err
	}
	return a.requireWorkspaceVisible(ctx, workspaceName)
}

func (a *API) requireDAGRunExecute(ctx context.Context, dagRun exec.DAGRunRef) error {
	if a.authService == nil {
		return nil
	}
	workspaceName, err := a.workspaceNameForDAGRun(ctx, dagRun)
	if err != nil {
		return err
	}
	return a.requireExecuteForWorkspace(ctx, workspaceName)
}
