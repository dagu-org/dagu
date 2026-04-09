// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  NodeStatus,
  NodeStatusLabel,
  Status,
  StatusLabel,
  TriggerType,
} from '@/api/v1/schema';
import { describe, expect, it } from 'vitest';
import {
  buildImproveDAGDefinitionPrompt,
  formatLatestDAGRunDetail,
} from '../improveDagDefinitionPrompt';

describe('improveDagDefinitionPrompt', () => {
  it('builds a prompt with the user request and latest run summary', () => {
    const prompt = buildImproveDAGDefinitionPrompt({
      dagFile: 'example.yaml',
      dagName: 'example',
      userPrompt: 'Make failures easier to debug.',
      latestDAGRun: {
        dagRunId: 'run-1234567890',
        name: 'example',
        status: Status.Failed,
        statusLabel: StatusLabel.failed,
        startedAt: '2026-04-09T10:00:00Z',
        finishedAt: '2026-04-09T10:01:00Z',
        rootDAGRunName: 'example',
        rootDAGRunId: 'run-1234567890',
        log: '/tmp/example.log',
        autoRetryCount: 0,
        nodes: [
          {
            step: { name: 'prepare' },
            stdout: '',
            stderr: '',
            startedAt: '2026-04-09T10:00:00Z',
            finishedAt: '2026-04-09T10:00:10Z',
            status: NodeStatus.Success,
            statusLabel: NodeStatusLabel.succeeded,
            retryCount: 0,
            doneCount: 1,
          },
          {
            step: { name: 'deploy' },
            stdout: '',
            stderr: '',
            startedAt: '2026-04-09T10:00:10Z',
            finishedAt: '2026-04-09T10:01:00Z',
            status: NodeStatus.Failed,
            statusLabel: NodeStatusLabel.failed,
            retryCount: 1,
            doneCount: 0,
            error: 'deployment timed out after waiting for readiness',
          },
        ],
        triggerType: TriggerType.manual,
      },
    });

    expect(prompt).toContain(
      'Improve the referenced DAG definition in the workspace.'
    );
    expect(prompt).toContain('DAG file: example.yaml');
    expect(prompt).toContain(
      'User request:\nMake failures easier to debug.'
    );
    expect(prompt).toContain('Run ID: run-1234567890');
    expect(prompt).toContain('Problematic steps:');
    expect(prompt).toContain(
      'deploy (failed) - deployment timed out after waiting for readiness'
    );
  });

  it('falls back cleanly when no latest run exists', () => {
    expect(formatLatestDAGRunDetail()).toBe(
      '- No latest run details are available for this DAG yet.'
    );
  });

  it('redacts sensitive params before adding them to the prompt summary', () => {
    const detail = formatLatestDAGRunDetail({
      dagRunId: 'run-2',
      name: 'example',
      status: Status.Success,
      statusLabel: StatusLabel.succeeded,
      startedAt: '2026-04-09T10:00:00Z',
      finishedAt: '2026-04-09T10:01:00Z',
      rootDAGRunName: 'example',
      rootDAGRunId: 'run-2',
      log: '/tmp/example.log',
      autoRetryCount: 0,
      params: JSON.stringify({
        username: 'alice',
        api_key: 'super-secret-value',
        nested: {
          access_token: 'abcdef',
        },
      }),
      nodes: [],
    });

    expect(detail).toContain('"api_key":"<REDACTED>"');
    expect(detail).toContain('"access_token":"<REDACTED>"');
    expect(detail).not.toContain('super-secret-value');
    expect(detail).not.toContain('abcdef');
  });
});
