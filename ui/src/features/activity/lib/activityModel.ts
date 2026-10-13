// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Status, type components } from '@/api/v1/schema';

export type DAGRunActivity = components['schemas']['DAGRunActivity'];
export type ActivityWorkflow = components['schemas']['DAGRunActivityWorkflow'];
export type ActivityRun = components['schemas']['DAGRunActivityRun'];

/** A time range in Unix seconds. */
export type TimeRange = { from: number; to: number };

/** Statuses that leave a workflow needing a look when its latest run ends so. */
const UNSUCCESSFUL = new Set<Status>([
  Status.Failed,
  Status.Rejected,
  Status.PartialSuccess,
  Status.Aborted,
]);

/** A running run is flagged once it takes this many times its usual time. */
export const SLOW_RUN_FACTOR = 2;

export type AttentionItem =
  | { kind: 'failing'; name: string; run: ActivityRun; failedCount: number }
  | { kind: 'waiting'; name: string; run: ActivityRun }
  | { kind: 'slow'; name: string; run: ActivityRun; usualSeconds: number };

function newestFirst(left: ActivityRun, right: ActivityRun): number {
  return (right.startedAt ?? 0) - (left.startedAt ?? 0);
}

/**
 * Lists what needs a person: workflows whose latest run did not succeed,
 * runs waiting for input, and runs taking much longer than usual.
 */
export function attentionItems(
  activity: DAGRunActivity,
  nowSeconds: number
): AttentionItem[] {
  const failing = activity.workflows
    .filter((workflow) => UNSUCCESSFUL.has(workflow.latest.status))
    .sort((left, right) => newestFirst(left.latest, right.latest))
    .map<AttentionItem>((workflow) => ({
      kind: 'failing',
      name: workflow.name,
      run: workflow.latest,
      failedCount: workflow.failedCount,
    }));

  const waiting = activity.active
    .filter((run) => run.status === Status.Waiting)
    .map<AttentionItem>((run) => ({
      kind: 'waiting',
      name: run.name ?? '',
      run,
    }));

  const usual = new Map(
    activity.workflows.map((workflow) => [
      workflow.name,
      workflow.medianSuccessSeconds,
    ])
  );
  const slow: AttentionItem[] = [];
  for (const run of activity.active) {
    const usualSeconds = usual.get(run.name ?? '');
    if (
      run.status !== Status.Running ||
      !usualSeconds ||
      run.startedAt === undefined
    ) {
      continue;
    }
    if (nowSeconds - run.startedAt > usualSeconds * SLOW_RUN_FACTOR) {
      slow.push({ kind: 'slow', name: run.name ?? '', run, usualSeconds });
    }
  }

  return [...failing, ...waiting, ...slow];
}

/** Runs that are running or queued; waiting runs are listed as attention. */
export function inProgressRuns(activity: DAGRunActivity): ActivityRun[] {
  return activity.active.filter((run) => run.status !== Status.Waiting);
}

function laneRank(workflow: ActivityWorkflow): number {
  if (UNSUCCESSFUL.has(workflow.latest.status)) return 0;
  if (workflow.strip.includes('r')) return 1;
  return 2;
}

/**
 * Orders lanes so unresolved failures come first, then workflows with runs
 * in progress, then the most recently active.
 */
export function sortLanes(workflows: ActivityWorkflow[]): ActivityWorkflow[] {
  return [...workflows].sort(
    (left, right) =>
      laneRank(left) - laneRank(right) ||
      newestFirst(left.latest, right.latest) ||
      left.name.localeCompare(right.name)
  );
}

/** Shortest range the lanes zoom out to when activity is sparse. */
const MIN_FIT_SECONDS = 60 * 60;

/**
 * Narrows the window to the part that has runs, keeping one empty bucket of
 * context before the first run and at least an hour of span.
 */
export function fitRange(activity: DAGRunActivity): TimeRange {
  const bucket = activity.bucketSeconds;
  let first = activity.to;
  for (const workflow of activity.workflows) {
    const index = workflow.strip.search(/[^.]/);
    if (index >= 0) {
      first = Math.min(first, activity.from + index * bucket);
    }
  }
  const from = Math.max(
    activity.from,
    Math.min(first - bucket, activity.to - MIN_FIT_SECONDS)
  );
  return { from, to: activity.to };
}

const TICK_STEPS = [
  60, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400,
];
const MIN_TICK_SPACING_PX = 72;

/**
 * Returns tick times for an axis of the given width, aligned to whole steps
 * in the server's time zone.
 */
export function timeTicks(
  range: TimeRange,
  widthPx: number,
  tzOffsetSeconds = 0
): number[] {
  const span = range.to - range.from;
  if (span <= 0 || widthPx <= 0) return [];
  const maxTicks = Math.max(1, Math.floor(widthPx / MIN_TICK_SPACING_PX));
  const step =
    TICK_STEPS.find((candidate) => span / candidate <= maxTicks) ??
    TICK_STEPS[TICK_STEPS.length - 1]!;
  const ticks: number[] = [];
  const firstLocal =
    Math.ceil((range.from + tzOffsetSeconds) / step) * step - tzOffsetSeconds;
  for (let tick = firstLocal; tick <= range.to; tick += step) {
    ticks.push(tick);
  }
  return ticks;
}

/** Position of a time inside a range, as a percentage. */
export function percentOf(range: TimeRange, seconds: number): number {
  return ((seconds - range.from) / (range.to - range.from)) * 100;
}
