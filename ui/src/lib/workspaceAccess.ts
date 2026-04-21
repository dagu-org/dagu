// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { components, UserRole, WorkspaceScope } from '@/api/v1/schema';

type WorkspaceAccess = components['schemas']['WorkspaceAccess'];
type User = components['schemas']['User'];

const ROLE_HIERARCHY: Record<UserRole, number> = {
  [UserRole.admin]: 5,
  [UserRole.manager]: 4,
  [UserRole.developer]: 3,
  [UserRole.operator]: 2,
  [UserRole.viewer]: 1,
};

export function normalizeAccess(access?: WorkspaceAccess): WorkspaceAccess {
  if (!access || access.all) {
    return { all: true, grants: [] };
  }
  return access;
}

export function effectiveWorkspaceRole(
  user: Pick<User, 'role' | 'workspaceAccess'> | null,
  workspace: string
): UserRole | null {
  if (!user) return null;
  if (!workspace) return user.role;
  const access = normalizeAccess(user.workspaceAccess);
  if (access.all) return user.role;
  return (
    access.grants.find((grant) => grant.workspace === workspace)?.role ?? null
  );
}

export function canAccessWorkspace(
  user: Pick<User, 'role' | 'workspaceAccess'> | null,
  workspace: string
): boolean {
  return effectiveWorkspaceRole(user, workspace) !== null;
}

export function roleAtLeast(role: UserRole | null, minimum: UserRole): boolean {
  if (!role) return false;
  return ROLE_HIERARCHY[role] >= ROLE_HIERARCHY[minimum];
}

export function workspaceRoleTarget(
  scope: WorkspaceScope | undefined,
  selectedWorkspace?: string | null
): string {
  if (scope === WorkspaceScope.workspace) {
    return selectedWorkspace || '';
  }
  return '';
}
