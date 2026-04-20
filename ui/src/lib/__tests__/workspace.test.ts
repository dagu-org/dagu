// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { WorkspaceScope } from '@/api/v1/schema';
import { beforeEach, describe, expect, it } from 'vitest';
import {
  getStoredWorkspaceSelection,
  LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY,
  WORKSPACE_SCOPE_STORAGE_KEY,
  WORKSPACE_STORAGE_KEY,
} from '../workspace';

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
