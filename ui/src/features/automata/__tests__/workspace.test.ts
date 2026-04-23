// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import {
  applySelectedWorkspaceToAutomataTags,
  automataMatchesSelectedWorkspace,
  filterAutomataBySelectedWorkspace,
  workspaceTagForAutomataSelection,
} from '../workspace';

describe('Automata workspace helpers', () => {
  it('builds the selected workspace tag using the shared workspace label format', () => {
    expect(workspaceTagForAutomataSelection('Engineering')).toBe(
      'workspace=Engineering'
    );
    expect(workspaceTagForAutomataSelection('team_alpha')).toBe(
      'workspace=team_alpha'
    );
    expect(workspaceTagForAutomataSelection('team-alpha')).toBe(
      'workspace=team-alpha'
    );
  });

  it('replaces existing workspace tags when creating inside a workspace', () => {
    expect(
      applySelectedWorkspaceToAutomataTags(
        ['owner=team-ai', 'workspace=old'],
        'Engineering'
      )
    ).toEqual(['owner=team-ai', 'workspace=Engineering']);
  });

  it('preserves tags when no workspace is selected', () => {
    expect(applySelectedWorkspaceToAutomataTags(['workspace=old'], '')).toEqual(
      ['workspace=old']
    );
  });

  it('matches automata tags case-insensitively for the selected workspace', () => {
    expect(
      automataMatchesSelectedWorkspace(
        { tags: ['owner=team-ai', 'workspace=engineering'] },
        'Engineering'
      )
    ).toBe(true);
    expect(
      automataMatchesSelectedWorkspace(
        { tags: ['workspace=ops'] },
        'Engineering'
      )
    ).toBe(false);
    expect(automataMatchesSelectedWorkspace({ tags: [] }, 'Engineering')).toBe(
      false
    );
  });

  it('filters the Automata list by the selected workspace tag', () => {
    const items = [
      { name: 'build', tags: ['workspace=engineering'] },
      { name: 'triage', tags: ['workspace=ops'] },
      { name: 'general', tags: ['owner=team-ai'] },
    ];

    expect(filterAutomataBySelectedWorkspace(items, '')).toEqual(items);
    expect(filterAutomataBySelectedWorkspace(items, 'Engineering')).toEqual([
      items[0],
    ]);
  });
});
