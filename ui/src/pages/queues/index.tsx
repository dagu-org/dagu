import { Layers, Search } from 'lucide-react';
import React from 'react';
import type { components } from '../../api/v2/schema';
import { Input } from '../../components/ui/input';
import { RefreshButton } from '../../components/ui/refresh-button';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { DAGRunDetailsModal } from '../../features/dag-runs/components/dag-run-details';
import QueueList from '../../features/queues/components/QueueList';
import QueueMetrics from '../../features/queues/components/QueueMetrics';
import { useQuery } from '../../hooks/api';
import { useQueuesListSSE } from '../../hooks/useQueuesListSSE';
import Title from '../../ui/Title';

function Queues() {
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  type QueueFilters = {
    searchText: string;
    queueType: string;
  };

  function areQueueFiltersEqual(a: QueueFilters, b: QueueFilters): boolean {
    return a.searchText === b.searchText && a.queueType === b.queueType;
  }

  const defaultFilters = React.useMemo<QueueFilters>(
    () => ({
      searchText: '',
      queueType: 'all',
    }),
    []
  );

  const [searchText, setSearchText] = React.useState(defaultFilters.searchText);
  const [selectedQueueType, setSelectedQueueType] = React.useState<string>(
    defaultFilters.queueType
  );

  const currentFilters = React.useMemo<QueueFilters>(
    () => ({
      searchText,
      queueType: selectedQueueType,
    }),
    [searchText, selectedQueueType]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef = React.useRef<QueueFilters | null>(null);

  React.useEffect(() => {
    const stored = searchState.readState<QueueFilters>('queues', remoteKey);
    const next = stored
      ? {
          searchText: stored.searchText ?? defaultFilters.searchText,
          queueType: stored.queueType ?? defaultFilters.queueType,
        }
      : defaultFilters;

    const current = currentFiltersRef.current;
    if (current && areQueueFiltersEqual(current, next)) {
      if (!stored) {
        searchState.writeState('queues', remoteKey, next);
      }
      lastPersistedFiltersRef.current = next;
      return;
    }

    setSearchText(next.searchText);
    setSelectedQueueType(next.queueType);
    lastPersistedFiltersRef.current = next;
    searchState.writeState('queues', remoteKey, next);
  }, [defaultFilters, remoteKey, searchState]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areQueueFiltersEqual(persisted, currentFilters)) {
      return;
    }
    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('queues', remoteKey, currentFilters);
  }, [currentFilters, remoteKey, searchState]);

  const [modalDAGRun, setModalDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);

  React.useEffect(() => {
    appBarContext.setTitle('Queue Dashboard');
  }, [appBarContext]);

  const sseResult = useQueuesListSSE(true);
  const usePolling = sseResult.shouldUseFallback || !sseResult.isConnected;

  const { data: pollingData, error, isLoading, mutate } = useQuery(
    '/queues',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: usePolling ? 3000 : 0,
      revalidateOnFocus: usePolling,
      revalidateOnReconnect: usePolling,
      isPaused: () => !usePolling,
    }
  );

  const data = sseResult.data || pollingData;

  async function handleRefresh(): Promise<void> {
    await mutate();
  }

  const filteredQueues = React.useMemo(() => {
    if (!data?.queues) return [];

    const search = searchText.toLowerCase();

    return data.queues
      .filter((queue) => {
        if (searchText && !queue.name.toLowerCase().includes(search)) {
          return false;
        }
        if (selectedQueueType !== 'all' && queue.type !== selectedQueueType) {
          return false;
        }
        return true;
      })
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [data?.queues, searchText, selectedQueueType]);

  const metrics = React.useMemo(() => {
    const queues = data?.queues || [];
    const globalQueuesList = queues.filter((q) => q.type === 'global');

    const totalRunning = queues.reduce((sum, q) => sum + (q.runningCount || 0), 0);
    const totalQueued = queues.reduce((sum, q) => sum + (q.queuedCount || 0), 0);

    const globalRunning = globalQueuesList.reduce((sum, q) => sum + (q.runningCount || 0), 0);
    const globalCapacity = globalQueuesList
      .filter((q) => q.maxConcurrency)
      .reduce((sum, q) => sum + (q.maxConcurrency || 0), 0);

    return {
      globalQueues: globalQueuesList.length,
      dagBasedQueues: queues.filter((q) => q.type === 'dag-based').length,
      activeQueues: queues.filter((q) => (q.runningCount || 0) > 0 || (q.queuedCount || 0) > 0).length,
      totalRunning,
      totalQueued,
      totalActive: totalRunning + totalQueued,
      utilization: globalCapacity > 0 ? Math.round((globalRunning / globalCapacity) * 100) : 0,
    };
  }, [data?.queues]);

  const handleDAGRunClick = React.useCallback(
    (dagRun: components['schemas']['DAGRunSummary']) => {
      setModalDAGRun({ name: dagRun.name, dagRunId: dagRun.dagRunId });
    },
    []
  );

  if (error) {
    const errorData = error as components['schemas']['Error'];
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center space-y-2">
          <Layers className="h-12 w-12 text-muted-foreground mx-auto" />
          <p className="text-sm text-muted-foreground">
            {errorData?.message || 'Failed to load queue information'}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2 w-full h-full overflow-hidden">
      <Title>Queues</Title>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 flex-shrink-0">
        <div className="flex flex-col sm:flex-row sm:items-center gap-3">
          <div className="flex items-center gap-2">
            <div className="relative">
              <Search className="absolute left-2 top-1.5 h-3 w-3 text-muted-foreground" />
              <Input
                type="text"
                placeholder="Search queues..."
                value={searchText}
                onChange={(e) => setSearchText(e.target.value)}
                className="h-7 w-[200px] pl-7 text-xs"
              />
            </div>
            <select
              value={selectedQueueType}
              onChange={(e) => setSelectedQueueType(e.target.value)}
              className="h-7 px-2 text-xs border border-border rounded bg-input"
            >
              <option value="all">All Types</option>
              <option value="global">Global</option>
              <option value="dag-based">DAG-based</option>
            </select>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {data?.queues && filteredQueues.length !== data.queues.length && (
            <span className="text-xs text-muted-foreground">
              ({filteredQueues.length} of {data.queues.length})
            </span>
          )}
          <RefreshButton onRefresh={handleRefresh} />
        </div>
      </div>

      <QueueMetrics metrics={metrics} isLoading={isLoading} />

      <div className="flex-1 min-h-0 overflow-auto">
        <QueueList
          queues={filteredQueues}
          isLoading={isLoading && !data}
          onDAGRunClick={handleDAGRunClick}
          onQueueCleared={handleRefresh}
        />
      </div>

      {modalDAGRun && (
        <DAGRunDetailsModal
          name={modalDAGRun.name}
          dagRunId={modalDAGRun.dagRunId}
          isOpen={!!modalDAGRun}
          onClose={() => setModalDAGRun(null)}
        />
      )}
    </div>
  );
}

export default Queues;
