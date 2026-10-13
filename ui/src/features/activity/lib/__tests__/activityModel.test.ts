// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { Status } from '@/api/v1/schema';
import {
  type ActivityWorkflow,
  attentionItems,
  type DAGRunActivity,
  fitRange,
  inProgressRuns,
  sortLanes,
  timeTicks,
} from '../activityModel';

const HOUR = 3600;
const FROM = 1_790_000_000 - (1_790_000_000 % 900);

function workflow(
  name: string,
  status: Status,
  startedAt: number,
  extra: Partial<ActivityWorkflow> = {}
): ActivityWorkflow {
  return {
    name,
    runCount: 1,
    failedCount: status === Status.Failed ? 1 : 0,
    strip: '.'.repeat(96),
    runs: [],
    latest: { dagRunId: `${name}-latest`, status, startedAt },
    ...extra,
  };
}

function activity(overrides: Partial<DAGRunActivity> = {}): DAGRunActivity {
  return {
    from: FROM,
    to: FROM + 24 * HOUR,
    bucketSeconds: 900,
    truncated: false,
    counts: [],
    workflows: [],
    active: [],
    upcoming: [],
    ...overrides,
  };
}

describe('attentionItems', () => {
  it('lists failing workflows newest first, then waiting and slow runs', () => {
    const now = FROM + 10 * HOUR;
    const items = attentionItems(
      activity({
        workflows: [
          workflow('older-failure', Status.Failed, FROM + HOUR),
          workflow('healthy', Status.Success, FROM + 2 * HOUR),
          workflow('newer-failure', Status.Failed, FROM + 3 * HOUR),
          workflow('train', Status.Running, FROM + 9 * HOUR, {
            medianSuccessSeconds: 600,
          }),
          workflow('quick', Status.Running, now - 60, {
            medianSuccessSeconds: 600,
          }),
        ],
        active: [
          { name: 'approve', dagRunId: 'a-1', status: Status.Waiting },
          {
            name: 'train',
            dagRunId: 't-1',
            status: Status.Running,
            startedAt: FROM + 9 * HOUR,
          },
          {
            name: 'quick',
            dagRunId: 'q-1',
            status: Status.Running,
            startedAt: now - 60,
          },
        ],
      }),
      now
    );

    expect(items.map((item) => `${item.kind}:${item.name}`)).toEqual([
      'failing:newer-failure',
      'failing:older-failure',
      'waiting:approve',
      'slow:train',
    ]);
  });
});

describe('inProgressRuns', () => {
  it('leaves waiting runs to the attention list', () => {
    const runs = inProgressRuns(
      activity({
        active: [
          { name: 'a', dagRunId: '1', status: Status.Running },
          { name: 'b', dagRunId: '2', status: Status.Waiting },
          { name: 'c', dagRunId: '3', status: Status.Queued },
        ],
      })
    );

    expect(runs.map((run) => run.dagRunId)).toEqual(['1', '3']);
  });
});

describe('sortLanes', () => {
  it('puts failing workflows first, then running ones, then recent activity', () => {
    const lanes = sortLanes([
      workflow('quiet', Status.Success, FROM + HOUR),
      workflow('busy', Status.Success, FROM + 5 * HOUR, {
        strip: 'r'.padEnd(96, '.'),
      }),
      workflow('broken', Status.Failed, FROM),
      workflow('recent', Status.Success, FROM + 6 * HOUR),
    ]);

    expect(lanes.map((lane) => lane.name)).toEqual([
      'broken',
      'busy',
      'recent',
      'quiet',
    ]);
  });
});

describe('fitRange', () => {
  it('starts one bucket before the first run', () => {
    const strip = '.'.repeat(40) + 's' + '.'.repeat(55);
    const range = fitRange(
      activity({ workflows: [workflow('a', Status.Success, 0, { strip })] })
    );

    expect(range).toEqual({ from: FROM + 39 * 900, to: FROM + 24 * HOUR });
  });

  it('keeps at least an hour in view', () => {
    const strip = '.'.repeat(95) + 's';
    const range = fitRange(
      activity({ workflows: [workflow('a', Status.Success, 0, { strip })] })
    );

    expect(range.to - range.from).toBe(HOUR);
  });
});

describe('timeTicks', () => {
  it('chooses whole-hour ticks that fit the width', () => {
    const ticks = timeTicks({ from: FROM, to: FROM + 12 * HOUR }, 600);

    expect(ticks.length).toBeLessThanOrEqual(600 / 72);
    expect(ticks.every((tick) => tick % HOUR === 0)).toBe(true);
  });

  it('aligns ticks to the server time zone', () => {
    const offset = 30 * 60;
    const ticks = timeTicks({ from: FROM, to: FROM + 6 * HOUR }, 800, offset);

    expect(ticks.every((tick) => (tick + offset) % HOUR === 0)).toBe(true);
  });
});
