// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { Status } from '@/api/v1/schema';
import {
  type ActivityRun,
  attentionItems,
  type DAGRunActivity,
  fitRange,
  groupLanes,
  inProgressRuns,
  isQueueStalled,
  isSchedulerHoldingWork,
  sortLanes,
  timeTicks,
} from '../activityModel';

const HOUR = 3600;
const FROM = 1_790_000_000 - (1_790_000_000 % 900);
const WINDOW = { from: FROM, to: FROM + 24 * HOUR };

function run(
  name: string,
  dagRunId: string,
  status: Status,
  startedAt: number,
  finishedAt?: number
): ActivityRun {
  return { name, dagRunId, status, startedAt, finishedAt };
}

function activity(overrides: Partial<DAGRunActivity> = {}): DAGRunActivity {
  return {
    ...WINDOW,
    runs: [],
    attention: [],
    active: [],
    upcoming: [],
    queues: [],
    ...overrides,
  };
}

describe('groupLanes', () => {
  it("keeps each workflow's runs newest first with the newest as latest", () => {
    const lanes = groupLanes([
      run('etl', 'etl-3', Status.Failed, FROM + 3 * HOUR),
      run('ping', 'ping-1', Status.Success, FROM + 2 * HOUR),
      run('etl', 'etl-2', Status.Success, FROM + HOUR),
      run('etl', 'etl-1', Status.Rejected, FROM),
    ]);

    expect(lanes).toEqual([
      expect.objectContaining({
        name: 'etl',
        latest: expect.objectContaining({ dagRunId: 'etl-3' }),
        failedCount: 2,
      }),
      expect.objectContaining({ name: 'ping', failedCount: 0 }),
    ]);
    expect(lanes[0]!.runs.map((item) => item.dagRunId)).toEqual([
      'etl-3',
      'etl-2',
      'etl-1',
    ]);
  });
});

describe('attentionItems', () => {
  it('lists failing workflows, then waiting runs, then slow runs', () => {
    const now = FROM + 10 * HOUR;
    // train usually takes 10 minutes; quick has too few runs to judge.
    const loaded = [
      run('train', 't-a', Status.Success, FROM, FROM + 600),
      run('train', 't-b', Status.Success, FROM + HOUR, FROM + HOUR + 500),
      run(
        'train',
        't-c',
        Status.Success,
        FROM + 2 * HOUR,
        FROM + 2 * HOUR + 700
      ),
      run('quick', 'q-a', Status.Success, FROM, FROM + 10),
    ];
    const items = attentionItems(
      activity({
        attention: [
          run('newer-failure', 'n-1', Status.Failed, FROM + 3 * HOUR),
          run('older-failure', 'o-1', Status.Failed, FROM + HOUR),
        ],
        active: [
          { name: 'approve', dagRunId: 'a-1', status: Status.Waiting },
          run('train', 't-1', Status.Running, FROM + 9 * HOUR),
          run('quick', 'q-1', Status.Running, FROM + 9 * HOUR),
        ],
      }),
      loaded,
      now
    );

    expect(items.map((item) => `${item.kind}:${item.name}`)).toEqual([
      'failing:newer-failure',
      'failing:older-failure',
      'waiting:approve',
      'slow:train',
    ]);
    expect(items[3]).toMatchObject({ usualSeconds: 600 });
  });
});

describe('inProgressRuns', () => {
  // Queued runs are summarized per queue, and waiting runs need attention.
  it('lists only running runs', () => {
    const runs = inProgressRuns(
      activity({
        active: [
          { name: 'a', dagRunId: '1', status: Status.Running },
          { name: 'b', dagRunId: '2', status: Status.Waiting },
          { name: 'c', dagRunId: '3', status: Status.Queued },
        ],
      })
    );

    expect(runs.map((item) => item.dagRunId)).toEqual(['1']);
  });
});

describe('isQueueStalled', () => {
  const now = FROM + HOUR;
  const queue = {
    name: 'default',
    running: 0,
    maxConcurrency: 2,
    queued: 3,
    nextQueuedAt: now - 10 * 60,
  };

  it('flags a queue whose next run waits while it has room', () => {
    expect(isQueueStalled(queue, now)).toBe(true);
  });

  it('accepts a long wait in a full queue or a short one', () => {
    expect(isQueueStalled({ ...queue, running: 2 }, now)).toBe(false);
    expect(isQueueStalled({ ...queue, nextQueuedAt: now - 60 }, now)).toBe(
      false
    );
  });
});

describe('isSchedulerHoldingWork', () => {
  const scheduled = activity({
    upcoming: [{ name: 'nightly', fileName: 'nightly', nextRun: FROM + HOUR }],
  });
  const queued = activity({
    queues: [{ name: 'default', running: 0, queued: 1 }],
  });

  it('flags a stopped scheduler while runs are scheduled or queued', () => {
    expect(isSchedulerHoldingWork(scheduled, false)).toBe(true);
    expect(isSchedulerHoldingWork(queued, false)).toBe(true);
  });

  // A node without schedules may run no scheduler on purpose, and an older
  // node does not report one at all.
  it('stays quiet without waiting work or a known stopped scheduler', () => {
    expect(isSchedulerHoldingWork(activity(), false)).toBe(false);
    expect(isSchedulerHoldingWork(scheduled, true)).toBe(false);
    expect(isSchedulerHoldingWork(scheduled, undefined)).toBe(false);
  });
});

describe('sortLanes', () => {
  it('puts failing workflows first, then running ones, then recent activity', () => {
    const lanes = sortLanes(
      groupLanes([
        run('recent', 'r-1', Status.Success, FROM + 6 * HOUR),
        run('busy', 'b-1', Status.Success, FROM + 5 * HOUR),
        run('quiet', 'q-1', Status.Success, FROM + HOUR),
        run('broken', 'x-1', Status.Failed, FROM),
      ]),
      [run('busy', 'b-0', Status.Running, FROM - HOUR)]
    );

    expect(lanes.map((lane) => lane.name)).toEqual([
      'broken',
      'busy',
      'recent',
      'quiet',
    ]);
  });
});

describe('fitRange', () => {
  it('starts a quarter hour before the oldest run', () => {
    const range = fitRange(WINDOW, [
      run('a', '2', Status.Success, FROM + 10 * HOUR),
      run('a', '1', Status.Success, FROM + 6 * HOUR),
    ]);

    expect(range).toEqual({ from: FROM + 6 * HOUR - 900, to: WINDOW.to });
  });

  it('keeps at least an hour in view', () => {
    const range = fitRange(WINDOW, [
      run('a', '1', Status.Success, WINDOW.to - 60),
    ]);

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
