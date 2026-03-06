import { useContext, useMemo } from 'react';
import { useQuery } from '@/hooks/api';
import { useDAGRunsListSSE } from '@/hooks/useDAGRunsListSSE';
import { sseFallbackOptions, useSSECacheSync } from '@/hooks/useSSECacheSync';
import { AppBarContext } from '@/contexts/AppBarContext';
import { components, Status } from '@/api/v1/schema';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

export interface KanbanColumns {
  queued: DAGRunSummary[];
  running: DAGRunSummary[];
  done: DAGRunSummary[];
  failed: DAGRunSummary[];
}

function dayBounds(dateStr: string): { fromDate: number; toDate: number } {
  const d = new Date(dateStr + 'T00:00:00');
  const fromDate = Math.floor(d.getTime() / 1000);
  const next = new Date(d);
  next.setDate(next.getDate() + 1);
  const toDate = Math.floor(next.getTime() / 1000);
  return { fromDate, toDate };
}

function groupByStatus(runs: DAGRunSummary[]): KanbanColumns {
  const columns: KanbanColumns = { queued: [], running: [], done: [], failed: [] };
  for (const run of runs) {
    switch (run.status) {
      case Status.Queued:
      case Status.NotStarted:
        columns.queued.push(run);
        break;
      case Status.Running:
        columns.running.push(run);
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

export function useDateKanbanData(
  date: string,
  selectedWorkspace: string,
  isToday: boolean
) {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const tag = selectedWorkspace ? `workspace=${selectedWorkspace}` : undefined;

  const { fromDate, toDate } = useMemo(() => dayBounds(date), [date]);

  // SSE only for today
  const sseParams = useMemo(
    () => ({ remoteNode, tags: tag, fromDate, toDate }),
    [remoteNode, tag, fromDate, toDate]
  );
  const sseResult = useDAGRunsListSSE(sseParams, isToday && !!selectedWorkspace);

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
    {
      ...(isToday ? sseFallbackOptions(sseResult) : { refreshInterval: 0 }),
      isPaused: () => !selectedWorkspace,
    }
  );

  // Always call the hook (rules of hooks), but SSE data is only present when isToday
  useSSECacheSync(sseResult, mutate);

  const columns = useMemo(
    () => groupByStatus(data?.dagRuns ?? []),
    [data?.dagRuns]
  );

  const isEmpty =
    columns.queued.length === 0 &&
    columns.running.length === 0 &&
    columns.done.length === 0 &&
    columns.failed.length === 0;

  return { columns, isLoading: !data, isEmpty };
}
