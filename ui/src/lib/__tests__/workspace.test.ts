// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it } from 'vitest';
import {
  ALL_WORKSPACES_DISPLAY_NAME,
  DEFAULT_WORKSPACE_DISPLAY_NAME,
  getStoredWorkspaceSelection,
  hasWorkspaceLabel,
  LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY,
  LEGACY_WORKSPACE_SCOPE_STORAGE_KEY,
  WORKSPACE_STORAGE_KEY,
  WorkspaceScope,
  workspaceSelectionLabel,
  workspaceTargetSelectionQuery,
} from '../workspace';

describe('workspace labels', () => {
  it('uses concise labels for aggregate and default scopes', () => {
    expect(workspaceSelectionLabel({ scope: WorkspaceScope.all })).toBe(
      ALL_WORKSPACES_DISPLAY_NAME
    );
    expect(workspaceSelectionLabel({ scope: WorkspaceScope.default })).toBe(
      DEFAULT_WORKSPACE_DISPLAY_NAME
    );
  });

  it('detects malformed workspace labels as workspace-labelled', () => {
    expect(hasWorkspaceLabel(['workspace=bad value'])).toBe(true);
    expect(hasWorkspaceLabel(['team=ops'])).toBe(false);
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
    expect(
      localStorage.getItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY)
    ).toBeNull();
    expect(
      JSON.parse(localStorage.getItem(WORKSPACE_STORAGE_KEY) ?? '')
    ).toEqual({
      scope: WorkspaceScope.workspace,
      workspace: 'team-a',
    });
  });

  it('migrates the deprecated workspace scope storage key', () => {
    localStorage.setItem(
      LEGACY_WORKSPACE_SCOPE_STORAGE_KEY,
      JSON.stringify({ scope: WorkspaceScope.default })
    );

    expect(getStoredWorkspaceSelection()).toEqual({
      scope: WorkspaceScope.default,
    });
    expect(localStorage.getItem(LEGACY_WORKSPACE_SCOPE_STORAGE_KEY)).toBeNull();
    expect(
      JSON.parse(localStorage.getItem(WORKSPACE_STORAGE_KEY) ?? '')
    ).toEqual({
      scope: WorkspaceScope.default,
    });
  });

  it('drops deprecated stored legacy scope names', () => {
    localStorage.setItem(
      LEGACY_WORKSPACE_SCOPE_STORAGE_KEY,
      JSON.stringify({ scope: 'none' })
    );

    expect(getStoredWorkspaceSelection()).toEqual({
      scope: WorkspaceScope.all,
    });
    expect(localStorage.getItem(LEGACY_WORKSPACE_SCOPE_STORAGE_KEY)).toBeNull();
    expect(localStorage.getItem(WORKSPACE_STORAGE_KEY)).toBeNull();
  });
});

describe('workspace target queries', () => {
  it('uses omitted workspace for default targets and workspace for named targets', () => {
    expect(workspaceTargetSelectionQuery({ scope: WorkspaceScope.all })).toBe(
      null
    );
    expect(
      workspaceTargetSelectionQuery({ scope: WorkspaceScope.default })
    ).toEqual({ workspace: WorkspaceScope.default });
    expect(
      workspaceTargetSelectionQuery({
        scope: WorkspaceScope.workspace,
        workspace: 'team-a',
      })
    ).toEqual({ workspace: 'team-a' });
  });
});
