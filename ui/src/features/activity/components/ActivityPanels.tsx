// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { ArrowRight, CircleCheck } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';
import { Status, StatusLabel, type components } from '@/api/v1/schema';
import { RelativeTime } from '@/components/ui/relative-time';
import StatusChip from '@/components/ui/status-chip';
import Ticker from '@/components/ui/ticker';
import { DAGRunActions } from '@/features/dag-runs/components/common';
import {
  buildDAGPageURL,
  buildDAGRunPageURL,
} from '@/features/dag-runs/lib/dagRunUrls';
import { I18nText } from '@/i18n/I18nText';
import { useI18n } from '@/i18n/I18nProvider';
import dayjs from '@/lib/dayjs';
import { formatDurationMs } from '@/lib/formatDuration';
import { runStatusText } from '@/lib/status-utils';
import { executionsURL } from '../lib/activityLinks';
import type {
  ActivityRun,
  AttentionItem,
  DAGRunActivity,
} from '../lib/activityModel';

type DAGRunSummary = components['schemas']['DAGRunSummary'];
type Upcoming = components['schemas']['DAGRunActivityUpcoming'];

/** Labels of the statuses a retry can start from. */
const RETRYABLE_LABELS: Partial<Record<Status, StatusLabel>> = {
  [Status.Failed]: StatusLabel.failed,
  [Status.Aborted]: StatusLabel.aborted,
  [Status.Rejected]: StatusLabel.rejected,
  [Status.PartialSuccess]: StatusLabel.partially_succeeded,
};

function isoTime(seconds?: number): string | undefined {
  return seconds === undefined ? undefined : dayjs.unix(seconds).toISOString();
}

/** The run fields the shared run actions read, built from activity data. */
function retrySummary(name: string, run: ActivityRun): DAGRunSummary | null {
  const statusLabel = RETRYABLE_LABELS[run.status];
  if (!statusLabel) return null;
  return {
    name,
    dagRunId: run.dagRunId,
    status: run.status,
    statusLabel,
    autoRetryCount: 0,
    startedAt: isoTime(run.startedAt) ?? '',
    finishedAt: isoTime(run.finishedAt) ?? '',
    artifactsAvailable: false,
  };
}

function SectionTitle({
  children,
  count,
}: {
  children: React.ReactNode;
  count?: number;
}): React.ReactElement {
  return (
    <h2 className="flex items-baseline gap-2 text-sm font-medium text-foreground">
      {children}
      {count !== undefined && (
        <span className="text-xs font-normal text-muted-foreground tabular-nums">
          {count}
        </span>
      )}
    </h2>
  );
}

function RunLink({
  name,
  run,
  remoteNode,
  children,
  className,
}: {
  name: string;
  run: ActivityRun;
  remoteNode: string;
  children: React.ReactNode;
  className?: string;
}): React.ReactElement {
  return (
    <Link
      to={buildDAGRunPageURL({
        rootDAGRunName: name,
        rootDAGRunId: run.dagRunId,
        remoteNode,
      })}
      className={className}
    >
      {children}
    </Link>
  );
}

/** What needs a person now; one quiet line when nothing does. */
export function NeedsAttention({
  items,
  remoteNode,
  onChanged,
}: {
  items: AttentionItem[];
  remoteNode: string;
  onChanged: () => void;
}): React.ReactElement {
  const { ts } = useI18n();
  if (items.length === 0) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <CircleCheck className="h-4 w-4 text-success" aria-hidden="true" />
        <I18nText
          text={'All clear. No failing workflows or runs waiting for input.'}
        />
      </p>
    );
  }
  return (
    <section
      className="flex flex-col gap-2"
      aria-labelledby="needs-attention-title"
    >
      <div id="needs-attention-title">
        <SectionTitle count={items.length}>
          <I18nText text={'Needs attention'} />
        </SectionTitle>
      </div>
      <ul className="divide-y divide-border rounded-lg border border-border bg-card">
        {items.map((item) => {
          const summary =
            item.kind === 'failing' ? retrySummary(item.name, item.run) : null;
          return (
            <li
              key={`${item.kind}:${item.name}:${item.run.dagRunId}`}
              className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2"
            >
              <StatusChip status={item.run.status} size="xs">
                {runStatusText(item.run.status)}
              </StatusChip>
              <RunLink
                name={item.name}
                run={item.run}
                remoteNode={remoteNode}
                className="text-sm font-medium text-foreground hover:underline"
              >
                {item.name}
              </RunLink>
              <span className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
                {item.kind === 'failing' &&
                  (item.run.failedStep
                    ? `${item.run.failedStep}: ${item.run.error ?? ''}`
                    : (item.run.error ?? ''))}
                {item.kind === 'waiting' && ts('Waiting for input')}
                {item.kind === 'slow' && (
                  <Ticker intervalMs={1000}>
                    {() => (
                      <span>
                        {ts('Running {elapsed}, usually {usual}', {
                          elapsed: formatDurationMs(
                            Date.now() - (item.run.startedAt ?? 0) * 1000
                          ),
                          usual: formatDurationMs(item.usualSeconds * 1000),
                        })}
                      </span>
                    )}
                  </Ticker>
                )}
              </span>
              {item.kind === 'failing' && item.failedCount > 1 && (
                <span className="text-xs text-error tabular-nums">
                  {ts('{count} failed', { count: item.failedCount })}
                </span>
              )}
              <RelativeTime
                timestamp={isoTime(item.run.startedAt)}
                compact
                className="text-xs text-muted-foreground"
              />
              <div className="flex items-center gap-1">
                {summary && (
                  <DAGRunActions
                    dagRun={summary}
                    name={item.name}
                    refresh={onChanged}
                    hideUnavailable
                  />
                )}
                <RunLink
                  name={item.name}
                  run={item.run}
                  remoteNode={remoteNode}
                  className="inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  {item.kind === 'waiting' ? ts('Review') : ts('Open')}
                  <ArrowRight className="h-3 w-3" aria-hidden="true" />
                </RunLink>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/** Runs that are running or queued, oldest first, with live elapsed time. */
export function InProgress({
  runs,
  remoteNode,
}: {
  runs: ActivityRun[];
  remoteNode: string;
}): React.ReactElement {
  return (
    <section className="flex flex-col gap-2">
      <SectionTitle count={runs.length}>
        <I18nText text={'In progress'} />
      </SectionTitle>
      {runs.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          <I18nText text={'Nothing is running.'} />
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {runs.map((run) => (
            <li key={run.dagRunId} className="flex items-center gap-2 text-sm">
              <StatusChip status={run.status} size="xs">
                {runStatusText(run.status)}
              </StatusChip>
              <RunLink
                name={run.name ?? ''}
                run={run}
                remoteNode={remoteNode}
                className="min-w-0 flex-1 truncate text-foreground hover:underline"
              >
                {run.name}
              </RunLink>
              {run.status === Status.Running && run.startedAt !== undefined && (
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                  <Ticker intervalMs={1000}>
                    {() => (
                      <span>
                        {formatDurationMs(
                          Date.now() - (run.startedAt ?? 0) * 1000
                        )}
                      </span>
                    )}
                  </Ticker>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** The next scheduled runs with a countdown to each. */
export function UpNext({
  upcoming,
  remoteNode,
  tzOffsetSeconds = 0,
}: {
  upcoming: Upcoming[];
  remoteNode: string;
  tzOffsetSeconds?: number;
}): React.ReactElement {
  return (
    <section className="flex flex-col gap-2">
      <SectionTitle>
        <I18nText text={'Up next'} />
      </SectionTitle>
      {upcoming.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          <I18nText text={'No scheduled runs.'} />
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {upcoming.map((item) => (
            <li key={item.fileName} className="flex items-center gap-2 text-sm">
              <Link
                to={buildDAGPageURL({ fileName: item.fileName, remoteNode })}
                className="min-w-0 flex-1 truncate text-foreground hover:underline"
              >
                {item.name}
              </Link>
              <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                {dayjs
                  .unix(item.nextRun)
                  .utcOffset(tzOffsetSeconds / 60)
                  .format('HH:mm')}
              </span>
              <span className="w-16 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                <Ticker intervalMs={1000}>
                  {() => (
                    <span>
                      {formatDurationMs(
                        Math.max(0, item.nextRun * 1000 - Date.now())
                      )}
                    </span>
                  )}
                </Ticker>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** Statuses in the order the counts row lists them. */
const COUNT_ORDER = [
  Status.Success,
  Status.Failed,
  Status.Running,
  Status.Waiting,
  Status.Queued,
  Status.Aborted,
  Status.PartialSuccess,
  Status.Rejected,
  Status.NotStarted,
];

/** Exact run counts for the window, each linking to the matching runs. */
export function ActivityCounts({
  activity,
}: {
  activity: DAGRunActivity;
}): React.ReactElement {
  const { ts } = useI18n();
  const counts = new Map(
    activity.counts.map((count) => [count.status, count.count])
  );
  const total = activity.counts.reduce((sum, count) => sum + count.count, 0);
  const range = { from: activity.from, to: activity.to };
  return (
    <div className="flex flex-wrap items-baseline gap-x-5 gap-y-1">
      <Link
        to={executionsURL(range)}
        className="group flex items-baseline gap-1.5"
      >
        <span className="text-xl font-semibold text-foreground tabular-nums group-hover:underline">
          {total.toLocaleString()}
        </span>
        <span className="text-xs text-muted-foreground">
          <I18nText text={'runs'} />
        </span>
      </Link>
      {COUNT_ORDER.filter(
        (status) =>
          (counts.get(status) ?? 0) > 0 ||
          status === Status.Success ||
          status === Status.Failed
      ).map((status) => (
        <Link
          key={status}
          to={executionsURL({ ...range, status })}
          className="group flex items-baseline gap-1.5"
        >
          <span
            className={
              status === Status.Failed && (counts.get(status) ?? 0) > 0
                ? 'text-xl font-semibold text-error tabular-nums group-hover:underline'
                : 'text-xl font-semibold text-foreground tabular-nums group-hover:underline'
            }
          >
            {(counts.get(status) ?? 0).toLocaleString()}
          </span>
          <span className="text-xs text-muted-foreground">
            {ts(runStatusText(status)).toLowerCase()}
          </span>
        </Link>
      ))}
    </div>
  );
}
