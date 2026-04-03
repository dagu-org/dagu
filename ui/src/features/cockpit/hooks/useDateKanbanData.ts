import { useContext, useMemo } from 'react';
import { useLiveConnection } from '@/hooks/useAppLive';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import dayjs from '@/lib/dayjs';
import { components, Status } from '@/api/v1/schema';
import { useExactDAGRuns } from '@/features/dag-runs/hooks/dagRunPagination';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

export interface KanbanColumns {
  queued: DAGRunSummary[];
  running: DAGRunSummary[];
  review: DAGRunSummary[];
  done: DAGRunSummary[];
  failed: DAGRunSummary[];
}

function dayBounds(
  dateStr: string,
  tzOffsetInSec: number | undefined
): { fromDate: number; toDate: number } {
  const d =
    tzOffsetInSec !== undefined
      ? dayjs(dateStr).utcOffset(tzOffsetInSec / 60, true)
      : dayjs(dateStr);
  return {
    fromDate: d.startOf('day').unix(),
    toDate: d.add(1, 'day').startOf('day').unix(),
  };
}

function groupByStatus(runs: DAGRunSummary[]): KanbanColumns {
  const columns: KanbanColumns = {
    queued: [],
    running: [],
    review: [],
    done: [],
    failed: [],
  };
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

export function useDateKanbanData(
  date: string,
  selectedWorkspace: string,
  isToday: boolean,
  isLive: boolean
) {
  const appBarContext = useContext(AppBarContext);
  const { tzOffsetInSec } = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const tag = selectedWorkspace ? `workspace=${selectedWorkspace}` : undefined;

  const { fromDate, toDate } = useMemo(
    () => dayBounds(date, tzOffsetInSec),
    [date, tzOffsetInSec]
  );
  const dagRunsQuery = useMemo(
    () => ({
      remoteNode,
      tags: tag,
      fromDate,
      toDate,
    }),
    [fromDate, remoteNode, tag, toDate]
  );

  useLiveConnection(isToday);
  const { data, error, refresh } = useExactDAGRuns({
    query: dagRunsQuery,
    liveEnabled: isLive,
    fallbackIntervalMs: isToday ? 2000 : 0,
  });

  const typedError = useMemo(() => {
    if (!error) {
      return null;
    }
    if (error instanceof Error) {
      return error;
    }
    return new Error('Failed to load runs');
  }, [error]);

  const columns = useMemo(() => groupByStatus(data), [data]);

  const isEmpty =
    columns.queued.length === 0 &&
    columns.running.length === 0 &&
    columns.review.length === 0 &&
    columns.done.length === 0 &&
    columns.failed.length === 0;

  return {
    columns,
    error: typedError,
    isLoading: data.length === 0 && !typedError,
    isEmpty,
    retry: refresh,
  };
}
