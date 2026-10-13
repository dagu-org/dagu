// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Status } from '@/api/v1/schema';
import { buildDAGRunPageURL } from '@/features/dag-runs/lib/dagRunUrls';
import { formatDurationMs } from '@/lib/formatDuration';
import dayjs from '@/lib/dayjs';
import { runStatusText } from '@/lib/status-utils';
import { cn } from '@/lib/utils';
import { useI18n } from '@/i18n/I18nProvider';
import {
  type ActivityRun,
  type ActivityWorkflow,
  type DAGRunActivity,
  percentOf,
  sortLanes,
  type TimeRange,
  timeTicks,
} from '../lib/activityModel';
import { executionsURL } from '../lib/activityLinks';

type Props = {
  activity: DAGRunActivity;
  range: TimeRange;
  nowSeconds: number;
  remoteNode: string;
  tzOffsetSeconds?: number;
  /** Narrows the window to a range dragged across the lanes. */
  onZoom: (range: TimeRange) => void;
};

/** Strip characters as the API defines them, with their run status. */
const STRIP_STATUS: Record<string, Status> = {
  s: Status.Success,
  r: Status.Running,
  p: Status.PartialSuccess,
  a: Status.Aborted,
  f: Status.Failed,
};

const STRIP_COLOR: Record<string, string> = {
  s: 'color-mix(in srgb, var(--status-success) 28%, transparent)',
  r: 'color-mix(in srgb, var(--status-running) 45%, transparent)',
  p: 'color-mix(in srgb, var(--status-warning) 55%, transparent)',
  a: 'color-mix(in srgb, var(--status-aborted) 55%, transparent)',
  f: 'color-mix(in srgb, var(--status-error) 70%, transparent)',
};

const MARK_COLOR: Partial<Record<Status, string>> = {
  [Status.Failed]: 'var(--status-error)',
  [Status.Rejected]: 'var(--status-error)',
  [Status.Aborted]: 'var(--status-aborted)',
  [Status.PartialSuccess]: 'var(--status-warning)',
  [Status.Running]: 'var(--status-running)',
  [Status.Waiting]: 'var(--status-running)',
  [Status.Queued]: 'var(--status-neutral)',
};

const PROBLEM_STATUSES = new Set<Status>([
  Status.Failed,
  Status.Rejected,
  Status.Aborted,
  Status.PartialSuccess,
]);

/** Pixels a pointer may travel before a press becomes a zoom drag. */
const DRAG_THRESHOLD_PX = 6;
/** How close, in pixels, a pointer must be to a run to pick it. */
const MARK_HIT_PX = 6;
/** Tick labels closer than this to an axis edge would be clipped. */
const TICK_EDGE_PERCENT = 3;

/**
 * Paints a lane's buckets as one gradient, merging neighboring buckets of
 * the same kind, so a lane costs one element however busy it is.
 */
export function stripGradient(
  strip: string,
  activity: DAGRunActivity,
  range: TimeRange
): string | undefined {
  const stops: string[] = [];
  let index = 0;
  while (index < strip.length) {
    const mark = strip[index]!;
    let end = index + 1;
    while (end < strip.length && strip[end] === mark) end++;
    const color = STRIP_COLOR[mark];
    if (color) {
      const start = percentOf(
        range,
        activity.from + index * activity.bucketSeconds
      );
      const stop = percentOf(
        range,
        activity.from + end * activity.bucketSeconds
      );
      if (stop > 0 && start < 100) {
        stops.push(
          `transparent ${start.toFixed(3)}%`,
          `${color} ${start.toFixed(3)}%`,
          `${color} ${stop.toFixed(3)}%`,
          `transparent ${stop.toFixed(3)}%`
        );
      }
    }
    index = end;
  }
  return stops.length
    ? `linear-gradient(to right, ${stops.join(', ')})`
    : undefined;
}

type Hover = {
  lane: string;
  clientX: number;
  clientY: number;
  run?: ActivityRun;
  bucket?: number;
};

type Drag = { lane: string; startX: number; currentX: number; rect: DOMRect };

function runAt(
  workflow: ActivityWorkflow,
  range: TimeRange,
  rect: DOMRect,
  clientX: number
): ActivityRun | undefined {
  let best: ActivityRun | undefined;
  let bestDistance = MARK_HIT_PX;
  for (const run of workflow.runs) {
    if (run.startedAt === undefined) continue;
    const x = rect.left + (percentOf(range, run.startedAt) / 100) * rect.width;
    const distance = Math.abs(x - clientX);
    if (distance <= bestDistance) {
      best = run;
      bestDistance = distance;
    }
  }
  return best;
}

function timeAt(range: TimeRange, rect: DOMRect, clientX: number): number {
  const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
  return range.from + ratio * (range.to - range.from);
}

/** One row per workflow: buckets of runs over time, with notable runs marked. */
export function ActivityLanes({
  activity,
  range,
  nowSeconds,
  remoteNode,
  tzOffsetSeconds = 0,
  onZoom,
}: Props): React.ReactElement {
  const { ts } = useI18n();
  const navigate = useNavigate();
  const axisRef = React.useRef<HTMLDivElement>(null);
  const [axisWidth, setAxisWidth] = React.useState(0);
  const [hover, setHover] = React.useState<Hover | null>(null);
  const [drag, setDrag] = React.useState<Drag | null>(null);
  const [focusedRun, setFocusedRun] = React.useState<{
    lane: string;
    index: number;
  } | null>(null);
  const lanes = React.useMemo(
    () => sortLanes(activity.workflows),
    [activity.workflows]
  );

  React.useLayoutEffect(() => {
    const element = axisRef.current;
    if (!element) return;
    const update = () => setAxisWidth(element.clientWidth);
    update();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const ticks = timeTicks(range, axisWidth, tzOffsetSeconds).filter((tick) => {
    const percent = percentOf(range, tick);
    return percent >= TICK_EDGE_PERCENT && percent <= 100 - TICK_EDGE_PERCENT;
  });
  const formatTime = (seconds: number) =>
    dayjs
      .unix(seconds)
      .utcOffset(tzOffsetSeconds / 60)
      .format('HH:mm');
  const nowPercent = percentOf(range, nowSeconds);

  const openRun = (name: string, run: ActivityRun, newTab: boolean) => {
    const url = buildDAGRunPageURL({
      rootDAGRunName: name,
      rootDAGRunId: run.dagRunId,
      remoteNode,
    });
    if (newTab) {
      window.open(url, '_blank', 'noopener');
    } else {
      navigate(url);
    }
  };

  const openBucket = (name: string, bucket: number) => {
    const from = activity.from + bucket * activity.bucketSeconds;
    navigate(executionsURL({ name, from, to: from + activity.bucketSeconds }));
  };

  const sortedRuns = (workflow: ActivityWorkflow) =>
    workflow.runs
      .filter(
        (run) =>
          run.startedAt !== undefined &&
          run.startedAt >= range.from &&
          run.startedAt <= range.to
      )
      .sort((left, right) => (left.startedAt ?? 0) - (right.startedAt ?? 0));

  const handlePointerMove = (
    workflow: ActivityWorkflow,
    event: React.PointerEvent<HTMLDivElement>
  ) => {
    const rect = event.currentTarget.getBoundingClientRect();
    if (drag && drag.lane === workflow.name) {
      setDrag({ ...drag, currentX: event.clientX });
      return;
    }
    const run = runAt(workflow, range, rect, event.clientX);
    const bucket = Math.floor(
      (timeAt(range, rect, event.clientX) - activity.from) /
        activity.bucketSeconds
    );
    setHover({
      lane: workflow.name,
      clientX: event.clientX,
      clientY: rect.top,
      run,
      bucket:
        workflow.strip[bucket] && workflow.strip[bucket] !== '.'
          ? bucket
          : undefined,
    });
  };

  const handlePointerUp = (
    workflow: ActivityWorkflow,
    event: React.PointerEvent<HTMLDivElement>
  ) => {
    const current = drag;
    setDrag(null);
    if (!current) return;
    if (Math.abs(current.currentX - current.startX) >= DRAG_THRESHOLD_PX) {
      const a = timeAt(range, current.rect, current.startX);
      const b = timeAt(range, current.rect, current.currentX);
      onZoom({
        from: Math.floor(Math.min(a, b)),
        to: Math.ceil(Math.max(a, b)),
      });
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const run = runAt(workflow, range, rect, event.clientX);
    const newTab = event.metaKey || event.ctrlKey;
    if (run) {
      openRun(workflow.name, run, newTab);
      return;
    }
    const bucket = Math.floor(
      (timeAt(range, rect, event.clientX) - activity.from) /
        activity.bucketSeconds
    );
    if (workflow.strip[bucket] && workflow.strip[bucket] !== '.') {
      openBucket(workflow.name, bucket);
    }
  };

  const handleKeyDown = (
    workflow: ActivityWorkflow,
    event: React.KeyboardEvent<HTMLDivElement>
  ) => {
    const runs = sortedRuns(workflow);
    if (runs.length === 0) return;
    const current =
      focusedRun?.lane === workflow.name ? focusedRun.index : runs.length;
    let next = current;
    if (event.key === 'ArrowRight')
      next = Math.min(runs.length - 1, current + 1);
    else if (event.key === 'ArrowLeft') next = Math.max(0, current - 1);
    else if (event.key === 'End') next = runs.length - 1;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'Enter' && focusedRun?.lane === workflow.name) {
      const run = runs[current];
      if (run) openRun(workflow.name, run, event.metaKey || event.ctrlKey);
      return;
    } else if (event.key === 'Escape') {
      setFocusedRun(null);
      setHover(null);
      return;
    } else return;
    event.preventDefault();
    if (next === current && focusedRun?.lane === workflow.name) return;
    const run = runs[next]!;
    const rect = event.currentTarget.getBoundingClientRect();
    setFocusedRun({ lane: workflow.name, index: next });
    setHover({
      lane: workflow.name,
      clientX:
        rect.left +
        (percentOf(range, run.startedAt ?? range.from) / 100) * rect.width,
      clientY: rect.top,
      run,
    });
  };

  const tooltip = hover ? (
    <div
      role="tooltip"
      className="pointer-events-none fixed z-50 -translate-x-1/2 -translate-y-full rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-md"
      style={{ left: hover.clientX, top: hover.clientY - 6 }}
    >
      <div className="font-medium">{hover.lane}</div>
      {hover.run ? (
        <div className="text-muted-foreground tabular-nums">
          {ts(runStatusText(hover.run.status))} ·{' '}
          {formatTime(hover.run.startedAt ?? 0)}
          {hover.run.finishedAt && hover.run.startedAt
            ? ` · ${formatDurationMs((hover.run.finishedAt - hover.run.startedAt) * 1000)}`
            : ''}
        </div>
      ) : hover.bucket !== undefined ? (
        <div className="text-muted-foreground tabular-nums">
          {formatTime(activity.from + hover.bucket * activity.bucketSeconds)}–
          {formatTime(
            activity.from + (hover.bucket + 1) * activity.bucketSeconds
          )}{' '}
          ·{' '}
          {ts(
            runStatusText(
              STRIP_STATUS[
                lanes.find((lane) => lane.name === hover.lane)?.strip[
                  hover.bucket
                ] ?? 's'
              ]!
            )
          )}
        </div>
      ) : null}
    </div>
  ) : null;

  return (
    <div className="min-w-0">
      <div className="grid grid-cols-1 border-b border-border sm:grid-cols-[minmax(7rem,13rem)_1fr]">
        <div className="hidden sm:block" />
        <div
          ref={axisRef}
          className="relative h-6 overflow-hidden text-[11px] text-muted-foreground tabular-nums"
          aria-hidden="true"
        >
          {ticks.map((tick) => (
            <span
              key={tick}
              className="absolute top-1 -translate-x-1/2 whitespace-nowrap"
              style={{ left: `${percentOf(range, tick)}%` }}
            >
              {formatTime(tick)}
            </span>
          ))}
        </div>
      </div>
      <ul aria-label={ts('Runs by workflow')}>
        {lanes.map((workflow) => {
          const visibleRuns = sortedRuns(workflow);
          const focused =
            focusedRun?.lane === workflow.name
              ? visibleRuns[focusedRun.index]
              : undefined;
          const dragRect =
            drag?.lane === workflow.name
              ? {
                  left: Math.min(drag.startX, drag.currentX) - drag.rect.left,
                  width: Math.abs(drag.currentX - drag.startX),
                }
              : null;
          return (
            <li
              key={workflow.name}
              className="grid grid-cols-1 items-stretch border-b border-border/60 last:border-b-0 sm:grid-cols-[minmax(7rem,13rem)_1fr]"
              style={{
                contentVisibility: 'auto',
                containIntrinsicSize: 'auto 2.25rem',
              }}
            >
              <div className="flex min-w-0 items-center gap-2 pt-1.5 pr-3 sm:py-1.5">
                <Link
                  to={executionsURL({
                    name: workflow.name,
                    from: activity.from,
                    to: activity.to,
                  })}
                  className="truncate text-sm font-medium text-foreground hover:underline"
                  title={workflow.name}
                >
                  {workflow.name}
                </Link>
                {workflow.failedCount > 0 ? (
                  <span className="shrink-0 text-xs font-medium text-error tabular-nums">
                    {ts('{failed} of {count} failed', {
                      failed: workflow.failedCount,
                      count: workflow.runCount,
                    })}
                  </span>
                ) : (
                  <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                    {ts(
                      workflow.runCount === 1 ? '{count} run' : '{count} runs',
                      {
                        count: workflow.runCount,
                      }
                    )}
                  </span>
                )}
              </div>
              <div
                role="group"
                tabIndex={0}
                aria-label={ts(
                  '{name}: {count} runs. Use the arrow keys to move between notable runs.',
                  { name: workflow.name, count: workflow.runCount }
                )}
                className="relative h-9 cursor-pointer touch-pan-y select-none outline-none focus-visible:ring-1 focus-visible:ring-ring"
                style={{
                  backgroundImage: stripGradient(
                    workflow.strip,
                    activity,
                    range
                  ),
                  backgroundSize: '100% 60%',
                  backgroundPosition: 'center',
                  backgroundRepeat: 'no-repeat',
                }}
                onPointerDown={(event) => {
                  if (event.button !== 0) return;
                  event.currentTarget.setPointerCapture?.(event.pointerId);
                  setDrag({
                    lane: workflow.name,
                    startX: event.clientX,
                    currentX: event.clientX,
                    rect: event.currentTarget.getBoundingClientRect(),
                  });
                }}
                onPointerMove={(event) => handlePointerMove(workflow, event)}
                onPointerUp={(event) => handlePointerUp(workflow, event)}
                onPointerCancel={() => setDrag(null)}
                onPointerLeave={() => {
                  if (!drag) setHover(null);
                }}
                onKeyDown={(event) => handleKeyDown(workflow, event)}
                onBlur={() => {
                  setFocusedRun(null);
                  setHover(null);
                }}
              >
                {visibleRuns.map((run) => {
                  const problem = PROBLEM_STATUSES.has(run.status);
                  return (
                    <span
                      key={run.dagRunId}
                      aria-hidden="true"
                      className={cn(
                        'absolute -translate-x-1/2 rounded-sm',
                        problem
                          ? 'top-0.5 bottom-0.5 w-[3px]'
                          : 'top-2 bottom-2 w-0.5',
                        focused === run &&
                          'ring-2 ring-ring ring-offset-1 ring-offset-background'
                      )}
                      style={{
                        left: `${percentOf(range, run.startedAt ?? range.from)}%`,
                        backgroundColor:
                          MARK_COLOR[run.status] ?? 'var(--status-success)',
                      }}
                    />
                  );
                })}
                {nowPercent >= 0 && nowPercent <= 100 && (
                  <span
                    aria-hidden="true"
                    className="absolute inset-y-0 w-px bg-primary/60"
                    style={{ left: `${nowPercent}%` }}
                  />
                )}
                {dragRect && dragRect.width >= DRAG_THRESHOLD_PX && (
                  <span
                    aria-hidden="true"
                    className="absolute inset-y-0 bg-primary/15 ring-1 ring-primary/40"
                    style={{ left: dragRect.left, width: dragRect.width }}
                  />
                )}
              </div>
            </li>
          );
        })}
      </ul>
      {tooltip}
    </div>
  );
}
