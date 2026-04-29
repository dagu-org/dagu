// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { updateControllerMetadataInSpec } from '../spec';

const baseMetadata = {
  description: '',
  iconUrl: '',
  goal: '',
  model: '',
  triggerPrompt: '',
  resetOnFinish: false,
  triggerType: 'manual' as const,
  cronSchedules: [] as string[],
};

describe('updateControllerMetadataInSpec', () => {
  it('writes workflow names to the workflows field', () => {
    const next = updateControllerMetadataInSpec('goal: "Ship it"\n', {
      ...baseMetadata,
      goal: 'Ship it',
      workflowNames: ['deploy', 'build'],
    });

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      workflows: {
        names: ['deploy', 'build'],
      },
    });
  });

  it('adds description when missing', () => {
    const next = updateControllerMetadataInSpec(
      'goal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Handles delivery work',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      goal: 'Ship it',
      workflows: { names: ['build'] },
    });
  });

  it('updates an existing description', () => {
    const next = updateControllerMetadataInSpec(
      'description: "Controller workflow"\ngoal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
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
    const next = updateControllerMetadataInSpec(
      'description: "Controller workflow"\ngoal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: '   ',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      goal: 'Ship it',
      workflows: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('description');
  });

  it('preserves unrelated fields', () => {
    const next = updateControllerMetadataInSpec(
      [
        'goal: "Ship it"',
        'labels:',
        '  - "team=platform"',
        'trigger:',
        '  type: "cron"',
        '  schedules:',
        '    - "* * * * *"',
        'workflows:',
        '  names:',
        '    - "build"',
        'agent:',
        '  safeMode: true',
        'disabled: false',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        triggerType: 'cron',
        description: 'Handles delivery work',
        iconUrl: 'https://cdn.example.com/icon.png',
        goal: 'Ship it',
        triggerPrompt: 'Handle the scheduled delivery cycle.',
        cronSchedules: ['* * * * *'],
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Handles delivery work',
      icon_url: 'https://cdn.example.com/icon.png',
      goal: 'Ship it',
      labels: ['team=platform'],
      trigger: {
        type: 'cron',
        prompt: 'Handle the scheduled delivery cycle.',
        schedules: ['* * * * *'],
      },
      workflows: { names: ['build'] },
      agent: { safeMode: true },
      disabled: false,
    });
  });

  it('removes icon url when blank', () => {
    const next = updateControllerMetadataInSpec(
      'description: "Controller workflow"\nicon_url: "https://cdn.example.com/old.png"\ngoal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Controller workflow',
        iconUrl: ' ',
        goal: 'Ship it',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Controller workflow',
      goal: 'Ship it',
    });
    expect(parse(next)).not.toHaveProperty('icon_url');
  });

  it('updates goal while preserving other metadata', () => {
    const next = updateControllerMetadataInSpec(
      'description: "Controller workflow"\nicon_url: "https://cdn.example.com/old.png"\ngoal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
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
      workflows: { names: ['build'] },
    });
  });

  it('removes goal when blank', () => {
    const next = updateControllerMetadataInSpec(
      'description: "Controller workflow"\ngoal: "Ship it"\nworkflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        description: 'Controller workflow',
        goal: '   ',
      }
    );

    expect(parse(next)).toMatchObject({
      description: 'Controller workflow',
      workflows: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('goal');
  });

  it('sets agent model while preserving other agent config', () => {
    const next = updateControllerMetadataInSpec(
      [
        'goal: "Ship it"',
        'workflows:',
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
    const next = updateControllerMetadataInSpec(
      [
        'goal: "Ship it"',
        'workflows:',
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
    expect(
      (parse(next) as { agent?: { model?: string } }).agent?.model
    ).toBeUndefined();
  });

  it('writes trigger prompt for cron controllers', () => {
    const next = updateControllerMetadataInSpec(
      'kind: "service"\nworkflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        triggerType: 'cron',
        cronSchedules: ['0 * * * *'],
        triggerPrompt:
          'Handle each scheduled cycle and work through the task list.',
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      trigger: {
        type: 'cron',
        schedules: ['0 * * * *'],
        prompt: 'Handle each scheduled cycle and work through the task list.',
      },
      workflows: { names: ['build'] },
    });
  });

  it('writes reset on finish', () => {
    const next = updateControllerMetadataInSpec(
      'workflows:\n  names:\n    - "build"\n',
      {
        ...baseMetadata,
        resetOnFinish: true,
      }
    );

    expect(parse(next)).toMatchObject({
      reset_on_finish: true,
    });
  });

  it('removes reset on finish when disabled', () => {
    const next = updateControllerMetadataInSpec(
      ['reset_on_finish: true', 'workflows:', '  names:', '    - "build"', ''].join(
        '\n'
      ),
      {
        ...baseMetadata,
        resetOnFinish: false,
      }
    );

    expect(parse(next)).not.toHaveProperty('reset_on_finish');
  });

  it('writes multi-line cron schedule expressions', () => {
    const metadata: Parameters<typeof updateControllerMetadataInSpec>[1] = {
      ...baseMetadata,
      triggerType: 'cron',
      triggerPrompt: 'Handle each scheduled cycle and work through the task list.',
      cronSchedules: ['0 * * * *', '30 9 * * 1-5'],
    };

    const next = updateControllerMetadataInSpec(
      'kind: "service"\nworkflows:\n  names:\n    - "build"\n',
      metadata
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      trigger: {
        type: 'cron',
        prompt: 'Handle each scheduled cycle and work through the task list.',
        schedules: ['0 * * * *', '30 9 * * 1-5'],
      },
    });
  });

  it('removes cron schedules when switching back to manual trigger', () => {
    const metadata: Parameters<typeof updateControllerMetadataInSpec>[1] = {
      ...baseMetadata,
      triggerType: 'manual',
    };

    const next = updateControllerMetadataInSpec(
      [
        'kind: "service"',
        'trigger:',
        '  type: "cron"',
        '  schedules:',
        '    - "0 * * * *"',
        '    - "30 9 * * 1-5"',
        '  prompt: "Handle each scheduled cycle and work through the task list."',
        'workflows:',
        '  names:',
        '    - "build"',
        '',
      ].join('\n'),
      metadata
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      trigger: {
        type: 'manual',
      },
      workflows: { names: ['build'] },
    });
    expect(parse(next)).not.toHaveProperty('trigger.schedules');
  });

  it('updates workflow names while preserving labels', () => {
    const next = updateControllerMetadataInSpec(
      [
        'kind: "service"',
        'workflows:',
        '  names:',
        '    - "build"',
        '  labels:',
        '    - "team=platform"',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        workflowNames: ['deploy', 'build', 'deploy', ' '],
      }
    );

    expect(parse(next)).toMatchObject({
      kind: 'service',
      workflows: {
        names: ['deploy', 'build'],
        labels: ['team=platform'],
      },
    });
  });

  it('removes workflow names when only labels remain', () => {
    const next = updateControllerMetadataInSpec(
      [
        'workflows:',
        '  names:',
        '    - "build"',
        '  labels:',
        '    - "team=platform"',
        '',
      ].join('\n'),
      {
        ...baseMetadata,
        workflowNames: [],
      }
    );

    expect(parse(next)).toMatchObject({
      workflows: {
        labels: ['team=platform'],
      },
    });
    expect(
      (parse(next) as { workflows?: { names?: string[] } }).workflows?.names
    ).toBeUndefined();
  });
});
