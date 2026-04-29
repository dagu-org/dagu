// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import {
  applySelectedWorkspaceToControllerTags,
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
      applySelectedWorkspaceToControllerTags(
        ['owner=team-ai', 'workspace=old'],
        'Engineering'
      )
    ).toEqual(['owner=team-ai', 'workspace=Engineering']);
  });

  it('preserves tags when no workspace is selected', () => {
    expect(applySelectedWorkspaceToControllerTags(['workspace=old'], '')).toEqual(
      ['workspace=old']
    );
  });

  it('matches controller tags case-insensitively for the selected workspace', () => {
    expect(
      controllerMatchesSelectedWorkspace(
        { tags: ['owner=team-ai', 'workspace=engineering'] },
        'Engineering'
      )
    ).toBe(true);
    expect(
      controllerMatchesSelectedWorkspace(
        { tags: ['workspace=ops'] },
        'Engineering'
      )
    ).toBe(false);
    expect(controllerMatchesSelectedWorkspace({ tags: [] }, 'Engineering')).toBe(
      false
    );
  });

  it('filters the Controller list by the selected workspace tag', () => {
    const items = [
      { name: 'build', tags: ['workspace=engineering'] },
      { name: 'triage', tags: ['workspace=ops'] },
      { name: 'general', tags: ['owner=team-ai'] },
    ];

    expect(filterControllerBySelectedWorkspace(items, '')).toEqual(items);
    expect(filterControllerBySelectedWorkspace(items, 'Engineering')).toEqual([
      items[0],
    ]);
  });
});
