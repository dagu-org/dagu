// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import {
  applySelectedWorkspaceToControllerLabels,
  controllerMatchesSelectedWorkspace,
  filterControllerBySelectedWorkspace,
  workspaceTagForControllerSelection,
} from '../workspace';

describe('Controller workspace helpers', () => {
  it('builds the selected workspace tag using the shared workspace label format', () => {
    expect(workspaceTagForControllerSelection('Engineering')).toBe(
      'workspace=Engineering'
    );
    expect(workspaceTagForControllerSelection('team_alpha')).toBe(
      'workspace=team_alpha'
    );
    expect(workspaceTagForControllerSelection('team-alpha')).toBe(
      'workspace=team-alpha'
    );
  });

  it('replaces existing workspace tags when creating inside a workspace', () => {
    expect(
      applySelectedWorkspaceToControllerLabels(
        ['owner=team-ai', 'workspace=old'],
        'Engineering'
      )
    ).toEqual(['owner=team-ai', 'workspace=Engineering']);
  });

  it('preserves tags when no workspace is selected', () => {
    expect(
      applySelectedWorkspaceToControllerLabels(['workspace=old'], '')
    ).toEqual(['workspace=old']);
  });

  it('matches controller labels case-insensitively for the selected workspace', () => {
    expect(
      controllerMatchesSelectedWorkspace(
        { labels: ['owner=team-ai', 'workspace=engineering'] },
        'Engineering'
      )
    ).toBe(true);
    expect(
      controllerMatchesSelectedWorkspace({ labels: ['workspace=ops'] }, 'Engineering')
    ).toBe(false);
    expect(controllerMatchesSelectedWorkspace({ labels: [] }, 'Engineering')).toBe(
      false
    );
  });

  it('filters the Controller list by the selected workspace label', () => {
    const items = [
      { name: 'build', labels: ['workspace=engineering'] },
      { name: 'triage', labels: ['workspace=ops'] },
      { name: 'general', labels: ['owner=team-ai'] },
    ];

    expect(filterControllerBySelectedWorkspace(items, '')).toEqual(items);
    expect(filterControllerBySelectedWorkspace(items, 'Engineering')).toEqual([
      items[0],
    ]);
  });
});
