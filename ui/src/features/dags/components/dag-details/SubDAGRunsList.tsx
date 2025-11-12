import { components } from '@/api/v2/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import dayjs from '@/lib/dayjs';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { useContext, useEffect } from 'react';
import { StatusDot } from '../common';

type SubDAGRun = components['schemas']['SubDAGRun'];
type SubDAGRunDetail = components['schemas']['SubDAGRunDetail'];
type IndexedSubRun = SubDAGRun & { originalIndex: number };
type IndexedSubRunDetail = SubDAGRunDetail & { originalIndex: number };
type SubRunListItem = IndexedSubRun | IndexedSubRunDetail;

type Props = {
  dagName: string;
  dagRunId: string;
  subDagName: string;
  allSubRuns: SubDAGRun[];
  isExpanded: boolean;
  onToggleExpand: () => void;
  onNavigate: (index: number, e?: React.MouseEvent) => void;
};

export function SubDAGRunsList({
  dagName,
  dagRunId,
  subDagName,
  allSubRuns,
  isExpanded,
  onToggleExpand,
  onNavigate,
}: Props) {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const dagRunIdsKey = allSubRuns.map((sr) => sr.dagRunId).join('|');

  // Fetch sub DAG run details with timing information
  // Only fetch if we have multiple sub runs AND expanded
  const shouldFetch = allSubRuns.length > 1 && isExpanded;
  const { data: subRunsData, mutate: refetchSubRuns } = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs',
    {
      params: {
        path: {
          name: dagName,
          dagRunId,
        },
        query: {
          remoteNode,
        },
      },
    },
    {
      isPaused: () => !shouldFetch,
      refreshInterval: shouldFetch ? 3000 : 0,
    }
  );

  // When the list of sub run ids changes (new executions), immediately refresh timing info
  useEffect(() => {
    if (!shouldFetch) {
      return;
    }
    refetchSubRuns();
  }, [shouldFetch, dagRunIdsKey, refetchSubRuns]);

  // Create a map of dagRunIds that belong to THIS node
  const nodeSubRunIds = new Set(allSubRuns.map((sr) => sr.dagRunId));

  // Map each sub run to include its original index
  const subRunsWithIndex: IndexedSubRun[] = allSubRuns.map((subRun, index) => ({
    ...subRun,
    originalIndex: index,
  }));

  // If we have API data with timing, filter to only THIS node's sub runs, merge and sort
  const subRunsFromApi: IndexedSubRunDetail[] = subRunsData?.subRuns
    ? subRunsData.subRuns
        // FILTER: Only include sub runs that belong to THIS node
        .filter((apiSubRun: SubDAGRunDetail) =>
          nodeSubRunIds.has(apiSubRun.dagRunId)
        )
        .map((apiSubRun): IndexedSubRunDetail => {
          const matchingSubRun = subRunsWithIndex.find(
            (sr) => sr.dagRunId === apiSubRun.dagRunId
          );
          return {
            ...apiSubRun,
            originalIndex: matchingSubRun?.originalIndex ?? 0,
          };
        })
        .sort((a, b) => {
          const timeA = new Date(a.startedAt).getTime();
          const timeB = new Date(b.startedAt).getTime();
          return timeB - timeA; // Descending order (newest first)
        })
    : [];

  const subRunsWithTiming: SubRunListItem[] =
    subRunsFromApi.length > 0 ? subRunsFromApi : subRunsWithIndex;

  if (allSubRuns.length === 1) {
    // Single sub DAG run
    return (
      <>
        <div
          className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline"
          onClick={(e) => {
            e.stopPropagation();
            onNavigate(0, e);
          }}
          title="Click to view sub DAG run (Cmd/Ctrl+Click to open in new tab)"
        >
          View Sub DAG Run: {subDagName}
        </div>
        {allSubRuns[0]?.params && (
          <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">
            Parameters:{' '}
            <span className="font-mono">{allSubRuns[0].params}</span>
          </div>
        )}
      </>
    );
  }

  // Multiple sub DAG runs
  return (
    <div className="text-xs text-slate-600 dark:text-slate-400 mt-1">
      <div className="flex items-center gap-1">
        <button
          onClick={(e) => {
            e.stopPropagation();
            onToggleExpand();
          }}
          className="flex items-center gap-1 text-blue-500 dark:text-blue-400 font-medium hover:underline"
        >
          {isExpanded ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronRight className="h-3 w-3" />
          )}
          Multiple executions: {allSubRuns.length} sub DAG runs
        </button>
      </div>
      {isExpanded && (
        <div className="mt-2 ml-4 space-y-1 border-l border-slate-200 dark:border-slate-700 pl-3">
          {subRunsWithTiming.map((subRun, displayIndex) => {
            const startedAt =
              'startedAt' in subRun ? subRun.startedAt : null;
            const status = 'status' in subRun ? subRun.status : null;
            const statusLabel =
              'statusLabel' in subRun ? subRun.statusLabel : undefined;
            // Display number: when sorted newest first, show descending numbers
            // So if we have 40 items, displayIndex 0 -> #40, displayIndex 1 -> #39, etc.
            const displayNumber = allSubRuns.length - displayIndex;
            return (
              <div key={subRun.dagRunId} className="py-1">
                <div className="flex items-center gap-2">
                  <div
                    className="text-xs text-blue-500 dark:text-blue-400 cursor-pointer hover:underline"
                    onClick={(e) => {
                      e.stopPropagation();
                      onNavigate(subRun.originalIndex, e);
                    }}
                    title="Click to view sub DAG run (Cmd/Ctrl+Click to open in new tab)"
                  >
                    #{displayNumber}: {subDagName}
                  </div>
                  {startedAt && (
                    <div className="text-xs text-slate-500 dark:text-slate-400">
                      {dayjs(startedAt).format('MMM D, HH:mm:ss')}
                    </div>
                  )}
                  {status && <StatusDot status={status} statusLabel={statusLabel} />}
                </div>
                {subRun.params && (
                  <div className="text-xs text-slate-500 dark:text-slate-400 ml-0 mt-1 font-mono">
                    {subRun.params}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
