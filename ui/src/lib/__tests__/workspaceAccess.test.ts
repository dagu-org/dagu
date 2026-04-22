// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { UserRole } from '@/api/v1/schema';
import { describe, expect, it } from 'vitest';
import { WorkspaceKind } from '../workspace';
import {
  effectiveWorkspaceRole,
  roleAtLeast,
  workspaceRoleTarget,
} from '../workspaceAccess';

describe('workspace access', () => {
  it('keeps aggregate workspace selection out of resource permission checks', () => {
    expect(workspaceRoleTarget({ kind: WorkspaceKind.all })).toBe('');
    expect(workspaceRoleTarget({ kind: WorkspaceKind.default })).toBe('');
    expect(
      workspaceRoleTarget({ kind: WorkspaceKind.workspace, workspace: 'ops' })
    ).toBe('ops');
  });

  it('uses workspace grants for concrete resources', () => {
    const user = {
      role: UserRole.viewer,
      workspaceAccess: {
        all: false,
        grants: [{ workspace: 'ops', role: UserRole.developer }],
      },
    };

    expect(
      roleAtLeast(effectiveWorkspaceRole(user, 'ops'), UserRole.developer)
    ).toBe(true);
    expect(effectiveWorkspaceRole(user, 'sales')).toBeNull();
  });

  it('treats missing grant arrays as no scoped grants', () => {
    const user = {
      role: UserRole.viewer,
      workspaceAccess: { all: false },
    } as Parameters<typeof effectiveWorkspaceRole>[0];

    expect(effectiveWorkspaceRole(user, 'ops')).toBeNull();
  });
});
