import { useCallback, useContext, useMemo } from 'react';
import { useQuery } from '@/hooks/api';
import { useDAGRunsListSSE } from '@/hooks/useDAGRunsListSSE';
import { sseFallbackOptions, useSSECacheSync } from '@/hooks/useSSECacheSync';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import dayjs from '@/lib/dayjs';
import { components, Status } from '@/api/v1/schema';
import type { KanbanColumns } from './useDateKanbanData';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

function groupByStatus(runs: DAGRunSummary[]): KanbanColumns {
  const columns: KanbanColumns = { queued: [], running: [], review: [], done: [], failed: [] };
  for (const run of runs) {
    switch (run.status) {
      case Status.Queued:
      case Status.NotStarted:
        columns.queued.push(run);
        break;
      case Status.Running:
        columns.running.push(run);
        break;
      case Status.Waiting:
        columns.review.push(run);
        break;
      case Status.Success:
      case Status.PartialSuccess:
        columns.done.push(run);
        break;
      case Status.Failed:
      case Status.Aborted:
      case Status.Rejected:
        columns.failed.push(run);
        break;
    }
  }
  return columns;
}

const EMPTY_COLUMNS: KanbanColumns = { queued: [], running: [], review: [], done: [], failed: [] };

/**
 * Fetches dag-runs for the full range of loaded dates in a SINGLE request
 * and partitions the results by date client-side. This replaces the old
 * per-date `useDateKanbanData` pattern that created N concurrent requests
 * (one per visible date), exhausting the browser's 6-connection limit.
 */
export function useCockpitDagRuns(
  loadedDates: string[],
  selectedWorkspace: string
) {
  const appBarContext = useContext(AppBarContext);
  const { tzOffsetInSec } = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const tag = selectedWorkspace ? `workspace=${selectedWorkspace}` : undefined;

  // Compute a single fromDate..toDate range covering all loaded dates
  const { fromDate, toDate } = useMemo(() => {
    if (loadedDates.length === 0) {
      const now = tzOffsetInSec !== undefined
        ? dayjs().utcOffset(tzOffsetInSec / 60)
        : dayjs();
      return {
        fromDate: now.startOf('day').unix(),
        toDate: now.add(1, 'day').startOf('day').unix(),
      };
    }
    const oldest = loadedDates[loadedDates.length - 1]!;
    const newest = loadedDates[0]!;

    const oldestDay = tzOffsetInSec !== undefined
      ? dayjs(oldest).utcOffset(tzOffsetInSec / 60, true)
      : dayjs(oldest);
    const newestDay = tzOffsetInSec !== undefined
      ? dayjs(newest).utcOffset(tzOffsetInSec / 60, true)
      : dayjs(newest);

    return {
      fromDate: oldestDay.startOf('day').unix(),
      toDate: newestDay.add(1, 'day').startOf('day').unix(),
    };
  }, [loadedDates, tzOffsetInSec]);

  // One SSE subscription for the full range
  const sseParams = useMemo(
    () => ({ remoteNode, tags: tag, fromDate, toDate }),
    [remoteNode, tag, fromDate, toDate]
  );
  const sseResult = useDAGRunsListSSE(sseParams, true);

  // One SWR fetch for the full range
  const { data, mutate } = useQuery(
    '/dag-runs',
    {
      params: {
        query: {
          remoteNode,
          tags: tag,
          fromDate,
          toDate,
        },
      },
    },
    sseFallbackOptions(sseResult)
  );
  useSSECacheSync(sseResult, mutate);

  const allRuns = data?.dagRuns ?? [];

  // Partition runs by date — memoized so each DateKanbanSection gets a stable reference
  const columnsByDate = useMemo(() => {
    const map = new Map<string, KanbanColumns>();
    if (allRuns.length === 0) return map;

    // Build a map of dateStr -> [fromTimestamp, toTimestamp]
    const dateBounds = new Map<string, { from: number; to: number }>();
    for (const dateStr of loadedDates) {
      const d = tzOffsetInSec !== undefined
        ? dayjs(dateStr).utcOffset(tzOffsetInSec / 60, true)
        : dayjs(dateStr);
      dateBounds.set(dateStr, {
        from: d.startOf('day').unix(),
        to: d.add(1, 'day').startOf('day').unix(),
      });
    }

    // Bucket each run into its date
    const buckets = new Map<string, DAGRunSummary[]>();
    for (const dateStr of loadedDates) {
      buckets.set(dateStr, []);
    }

    for (const run of allRuns) {
      const startedAt = run.startedAt;
      if (!startedAt) continue;
      const ts = dayjs(startedAt).unix();
      for (const dateStr of loadedDates) {
        const bounds = dateBounds.get(dateStr)!;
        if (ts >= bounds.from && ts < bounds.to) {
          buckets.get(dateStr)!.push(run);
          break;
        }
      }
    }

    for (const [dateStr, runs] of buckets) {
      map.set(dateStr, groupByStatus(runs));
    }
    return map;
  }, [allRuns, loadedDates, tzOffsetInSec]);

  const getColumnsForDate = useCallback(
    (date: string): KanbanColumns => columnsByDate.get(date) ?? EMPTY_COLUMNS,
    [columnsByDate]
  );

  return { getColumnsForDate, isLoading: !data };
}
