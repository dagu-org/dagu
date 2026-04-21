// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { WorkspaceScope } from '@/api/v1/schema';
import { beforeEach, describe, expect, it } from 'vitest';
import {
  ACCESSIBLE_WORKSPACES_DISPLAY_NAME,
  getStoredWorkspaceSelection,
  LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY,
  NO_WORKSPACE_DISPLAY_NAME,
  WORKSPACE_SCOPE_STORAGE_KEY,
  WORKSPACE_STORAGE_KEY,
  workspaceSelectionLabel,
} from '../workspace';

describe('workspace labels', () => {
  it('uses concise labels for aggregate and default scopes', () => {
    expect(workspaceSelectionLabel({ scope: WorkspaceScope.accessible })).toBe(
      ACCESSIBLE_WORKSPACES_DISPLAY_NAME
    );
    expect(workspaceSelectionLabel({ scope: WorkspaceScope.none })).toBe(
      NO_WORKSPACE_DISPLAY_NAME
    );
  });
});

describe('workspace storage', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('falls through to cockpit workspace migration when legacy storage is invalid', () => {
    localStorage.setItem(WORKSPACE_STORAGE_KEY, 'invalid workspace');
    localStorage.setItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY, 'team-a');

    expect(getStoredWorkspaceSelection()).toEqual({
      scope: WorkspaceScope.workspace,
      workspace: 'team-a',
    });
    expect(localStorage.getItem(WORKSPACE_STORAGE_KEY)).toBeNull();
    expect(
      localStorage.getItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY)
    ).toBeNull();
    expect(
      JSON.parse(localStorage.getItem(WORKSPACE_SCOPE_STORAGE_KEY) ?? '')
    ).toEqual({
      scope: WorkspaceScope.workspace,
      workspace: 'team-a',
    });
  });
});
