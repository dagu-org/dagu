// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { updateAutomataMetadataInSpec } from '../spec';

const baseMetadata = {
  description: '',
  iconUrl: '',
  goal: '',
  model: '',
  standingInstruction: '',
  schedule: [] as string[],
};

describe('updateAutomataMetadataInSpec', () => {
  it('adds description when missing', () => {
    const next = updateAutomataMetadataInSpec(
      'goal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Handles delivery work',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
      allowed_dags: { names: ['build'] },
    });
  });

  it('updates an existing description', () => {
    const next = updateAutomataMetadataInSpec(
      'description: "Automata workflow"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Handles delivery work',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
    });
  });

  it('removes description when blank', () => {
    const next = updateAutomataMetadataInSpec(
      'description: "Automata workflow"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: '   ',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      allowed_dags: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('description');
  });

  it('preserves unrelated fields', () => {
    const next = updateAutomataMetadataInSpec(
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
      {
        ...baseMetadata,
        description: 'Handles delivery work',
        iconUrl: 'https://cdn.example.com/icon.png',
        goal: 'Ship it',
        schedule: ['* * * * *'],
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      icon_url: 'https://cdn.example.com/icon.png',
      goal: 'Ship it',
      tags: ['team=platform'],
      schedule: '* * * * *',
      allowed_dags: { names: ['build'] },
      agent: { safeMode: true },
      disabled: false,
    });
  });

  it('removes icon url when blank', () => {
    const next = updateAutomataMetadataInSpec(
      'description: "Automata workflow"\nicon_url: "https://cdn.example.com/old.png"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Automata workflow',
        iconUrl: ' ',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Automata workflow',
      goal: 'Ship it',
    });
    expect(parse(next)).not.toHaveProperty('icon_url');
  });

  it('updates goal while preserving other metadata', () => {
    const next = updateAutomataMetadataInSpec(
      'description: "Automata workflow"\nicon_url: "https://cdn.example.com/old.png"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Handles delivery work',
        iconUrl: 'https://cdn.example.com/new.png',
        goal: 'Handle triage and delivery',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      icon_url: 'https://cdn.example.com/new.png',
      goal: 'Handle triage and delivery',
      allowed_dags: { names: ['build'] },
    });
  });

  it('removes goal when blank', () => {
    const next = updateAutomataMetadataInSpec(
      'description: "Automata workflow"\ngoal: "Ship it"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Automata workflow',
        goal: '   ',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Automata workflow',
      allowed_dags: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('goal');
  });

  it('sets agent model while preserving other agent config', () => {
    const next = updateAutomataMetadataInSpec(
      [
        'goal: "Ship it"',
        'allowed_dags:',
        '  names:',
        '    - "build"',
        'agent:',
        '  safeMode: true',
        '  soul: "default"',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        goal: 'Ship it',
        model: 'claude-sonnet-4-6',
      }
    );

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      agent: {
        model: 'claude-sonnet-4-6',
        safeMode: true,
        soul: 'default',
      },
    });
  });

  it('removes agent model and preserves remaining agent config', () => {
    const next = updateAutomataMetadataInSpec(
      [
        'goal: "Ship it"',
        'allowed_dags:',
        '  names:',
        '    - "build"',
        'agent:',
        '  model: "gpt-5-3-codex"',
        '  safeMode: true',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        goal: 'Ship it',
        model: ' ',
      }
    );

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      agent: {
        safeMode: true,
      },
    });
    expect((parse(next) as { agent?: { model?: string } }).agent?.model).toBeUndefined();
  });

  it('sets standing instruction for services', () => {
    const next = updateAutomataMetadataInSpec(
      'kind: "service"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        standingInstruction:
          'Handle each scheduled cycle and work through the task list.',
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      standing_instruction:
        'Handle each scheduled cycle and work through the task list.',
      allowed_dags: { names: ['build'] },
    });
  });

  it('removes standing instruction when blank', () => {
    const next = updateAutomataMetadataInSpec(
      [
        'kind: "service"',
        'standing_instruction: "Handle each scheduled cycle."',
        'allowed_dags:',
        '  names:',
        '    - "build"',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        standingInstruction: '   ',
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      allowed_dags: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('standing_instruction');
  });

  it('writes multi-line schedule expressions', () => {
    const next = updateAutomataMetadataInSpec(
      'kind: "service"\nallowed_dags:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        schedule: ['0 * * * *', '30 9 * * 1-5'],
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      schedule: ['0 * * * *', '30 9 * * 1-5'],
    });
  });

  it('removes schedule when blank', () => {
    const next = updateAutomataMetadataInSpec(
      [
        'kind: "service"',
        'schedule:',
        '  - "0 * * * *"',
        '  - "30 9 * * 1-5"',
        'allowed_dags:',
        '  names:',
        '    - "build"',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      allowed_dags: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('schedule');
  });
});
