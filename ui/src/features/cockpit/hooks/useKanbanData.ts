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

function sevenDaysAgoUnix(): number {
  return Math.floor((Date.now() - 7 * 24 * 60 * 60 * 1000) / 1000);
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

export function useKanbanData(selectedWorkspace: string) {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const tag = selectedWorkspace ? `workspace=${selectedWorkspace}` : undefined;
  const fromDate = useMemo(() => sevenDaysAgoUnix(), []);

  const sseParams = useMemo(
    () => ({ tags: tag, fromDate }),
    [tag, fromDate]
  );

  const sseResult = useDAGRunsListSSE(sseParams, !!selectedWorkspace);

  const { data, mutate } = useQuery(
    '/dag-runs',
    {
      params: {
        query: {
          remoteNode,
          tags: tag,
          fromDate,
        },
      },
    },
    {
      ...sseFallbackOptions(sseResult),
      isPaused: () => !selectedWorkspace,
    }
  );

  useSSECacheSync(sseResult, mutate);

  const columns = useMemo(
    () => groupByStatus(data?.dagRuns ?? []),
    [data?.dagRuns]
  );

  return { columns, isConnected: sseResult.isConnected };
}
