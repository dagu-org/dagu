// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { updateAutomataDescriptionInSpec } from '../spec';

describe('updateAutomataDescriptionInSpec', () => {
  it('adds description when missing', () => {
    const next = updateAutomataDescriptionInSpec(
      'goal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      'Handles delivery work'
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
      allowed_dags: { names: ['build'] },
    });
  });

  it('updates an existing description', () => {
    const next = updateAutomataDescriptionInSpec(
      'description: "Automata workflow"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      'Handles delivery work'
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
    });
  });

  it('removes description when blank', () => {
    const next = updateAutomataDescriptionInSpec(
      'description: "Automata workflow"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      '   '
    );

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      allowed_dags: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('description');
  });

  it('preserves unrelated fields', () => {
    const next = updateAutomataDescriptionInSpec(
      [
        'goal: "Ship it"',
        'tags:',
        '  - "team=platform"',
        'schedule: "* * * * *"',
        'allowed_dags:',
        '  names:',
        '    - "build"',
        'agent:',
        '  safeMode: true',
        'disabled: false',
        '',
      ].join('\n'),
      'Handles delivery work'
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
      tags: ['team=platform'],
      schedule: '* * * * *',
      allowed_dags: { names: ['build'] },
      agent: { safeMode: true },
      disabled: false,
    });
  });
});
