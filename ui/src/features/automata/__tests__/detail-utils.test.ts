// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';

import { AgentMessageType } from '@/api/v1/schema';
import {
  buildAutomataThread,
  formatAutomataScheduleText,
  parseAutomataScheduleText,
  validateAutomataScheduleExpressions,
} from '@/features/automata/detail-utils';

describe('buildAutomataThread', () => {
  it('merges queued and persisted messages in chronological order', () => {
    const thread = buildAutomataThread(
      [
        {
          id: 'm-2',
          type: AgentMessageType.assistant,
          createdAt: '2026-04-05T12:00:03Z',
          sequenceId: 2,
          sessionId: 'sess-1',
          cost: 0,
          content: 'Automata replied',
        },
      ],
      [
        {
          id: 'q-1',
          createdAt: '2026-04-05T12:00:01Z',
          kind: 'operator_message',
          message: 'Queued operator message',
        },
        {
          id: 'q-2',
          createdAt: '2026-04-05T12:00:02Z',
          kind: 'scheduled_tick',
          message: 'Scheduled tick',
        },
      ]
    );

    expect(thread.map((item) => item.id)).toEqual(['queued-q-1', 'queued-q-2', 'm-2']);
    expect(thread[0]).toMatchObject({
      kind: 'queued',
      queuedKind: 'operator_message',
    });
    expect(thread[2]).toMatchObject({
      kind: 'message',
      message: { content: 'Automata replied' },
    });
  });

  it('falls back to sequence order when message timestamps are identical', () => {
    const thread = buildAutomataThread(
      [
        {
          id: 'm-2',
          type: AgentMessageType.assistant,
          createdAt: '2026-04-05T12:00:03Z',
          sequenceId: 2,
          sessionId: 'sess-1',
          cost: 0,
          content: 'second',
        },
        {
          id: 'm-1',
          type: AgentMessageType.user,
          createdAt: '2026-04-05T12:00:03Z',
          sequenceId: 1,
          sessionId: 'sess-1',
          cost: 0,
          content: 'first',
        },
      ],
      []
    );

    expect(thread.map((item) => item.id)).toEqual(['m-1', 'm-2']);
  });
});

describe('automata schedule helpers', () => {
  it('normalizes newline-separated schedules', () => {
    const parsed = parseAutomataScheduleText('\n0 * * * *\n  \n30 9 * * 1-5\n');
    expect(parsed).toEqual(['0 * * * *', '30 9 * * 1-5']);
    expect(formatAutomataScheduleText(parsed)).toBe('0 * * * *\n30 9 * * 1-5');
  });

  it('validates cron expressions', () => {
    expect(validateAutomataScheduleExpressions(['0 * * * *'])).toBeNull();
    expect(validateAutomataScheduleExpressions(['not-a-cron'])).toContain(
      'Invalid cron schedule'
    );
  });
});
