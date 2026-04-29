// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import {
  applySelectedWorkspaceToAutopilotTags,
  autopilotMatchesSelectedWorkspace,
  filterAutopilotBySelectedWorkspace,
  workspaceTagForAutopilotSelection,
} from '../workspace';

describe('Autopilot workspace helpers', () => {
  it('builds the selected workspace tag using the shared workspace label format', () => {
    expect(workspaceTagForAutopilotSelection('Engineering')).toBe(
      'workspace=Engineering'
    );
    expect(workspaceTagForAutopilotSelection('team_alpha')).toBe(
      'workspace=team_alpha'
    );
    expect(workspaceTagForAutopilotSelection('team-alpha')).toBe(
      'workspace=team-alpha'
    );
  });

  it('replaces existing workspace tags when creating inside a workspace', () => {
    expect(
      applySelectedWorkspaceToAutopilotTags(
        ['owner=team-ai', 'workspace=old'],
        'Engineering'
      )
    ).toEqual(['owner=team-ai', 'workspace=Engineering']);
  });

  it('preserves tags when no workspace is selected', () => {
    expect(applySelectedWorkspaceToAutopilotTags(['workspace=old'], '')).toEqual(
      ['workspace=old']
    );
  });

  it('matches autopilot tags case-insensitively for the selected workspace', () => {
    expect(
      autopilotMatchesSelectedWorkspace(
        { tags: ['owner=team-ai', 'workspace=engineering'] },
        'Engineering'
      )
    ).toBe(true);
    expect(
      autopilotMatchesSelectedWorkspace(
        { tags: ['workspace=ops'] },
        'Engineering'
      )
    ).toBe(false);
    expect(autopilotMatchesSelectedWorkspace({ tags: [] }, 'Engineering')).toBe(
      false
    );
  });

  it('filters the Autopilot list by the selected workspace tag', () => {
    const items = [
      { name: 'build', tags: ['workspace=engineering'] },
      { name: 'triage', tags: ['workspace=ops'] },
      { name: 'general', tags: ['owner=team-ai'] },
    ];

    expect(filterAutopilotBySelectedWorkspace(items, '')).toEqual(items);
    expect(filterAutopilotBySelectedWorkspace(items, 'Engineering')).toEqual([
      items[0],
    ]);
  });
});
