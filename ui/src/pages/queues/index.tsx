import React from 'react';
import { Layers, Search } from 'lucide-react';
import { Input } from '../../components/ui/input';
import { RefreshButton } from '../../components/ui/refresh-button';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '../../components/ui/tooltip';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useQuery } from '../../hooks/api';
import type { components } from '../../api/v2/schema';
import QueueMetrics from '../../features/queues/components/QueueMetrics';
import QueueList from '../../features/queues/components/QueueList';
import { DAGRunDetailsModal } from '../../features/dag-runs/components/dag-run-details';
import Title from '../../ui/Title';

function Queues() {
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  type QueueFilters = {
    searchText: string;
    queueType: string;
  };

  const areQueueFiltersEqual = (a: QueueFilters, b: QueueFilters) =>
    a.searchText === b.searchText && a.queueType === b.queueType;

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

  // State for DAG run modal
  const [modalDAGRun, setModalDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);

  React.useEffect(() => {
    appBarContext.setTitle('Queue Dashboard');
  }, [appBarContext]);

  const { data, error, isLoading, mutate } = useQuery(
    '/queues',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: 3000, // Refresh every 3 seconds for real-time updates
      revalidateOnFocus: true,
      revalidateOnReconnect: true,
    }
  );

  const handleRefresh = async () => {
    await mutate();
  };

  // Filter queues based on search text and type
  const filteredQueues = React.useMemo(() => {
    if (!data?.queues) return [];

    let filtered = data.queues;

    // Filter by search text
    if (searchText) {
      const search = searchText.toLowerCase();
      filtered = filtered.filter((queue) =>
        queue.name.toLowerCase().includes(search)
      );
    }

    // Filter by queue type
    if (selectedQueueType !== 'all') {
      filtered = filtered.filter((queue) => queue.type === selectedQueueType);
    }

    // Sort alphabetically by queue name for stable display
    return filtered.sort((a, b) => a.name.localeCompare(b.name));
  }, [data?.queues, searchText, selectedQueueType]);

  // Calculate metrics
  const metrics = React.useMemo(() => {
    const queues = data?.queues || [];

    // Count queues by type
    const globalQueues = queues.filter((q) => q.type === 'global').length;
    const dagBasedQueues = queues.filter((q) => q.type === 'dag-based').length;

    // Count active queues (those with running or queued items)
    const activeQueues = queues.filter(
      (q) => (q.runningCount || 0) > 0 || (q.queuedCount || 0) > 0
    ).length;

    const totalRunning = queues.reduce(
      (sum, q) => sum + (q.runningCount || 0),
      0
    );
    const totalQueued = queues.reduce(
      (sum, q) => sum + (q.queuedCount || 0),
      0
    );
    const totalActive = totalRunning + totalQueued;

    // Calculate utilization for global queues only (DAG-based queues are isolated and don't compete for shared capacity)
    const globalQueuesList = queues.filter((q) => q.type === 'global');
    const globalRunning = globalQueuesList.reduce(
      (sum, q) => sum + (q.runningCount || 0),
      0
    );
    const globalCapacity = globalQueuesList
      .filter((q) => q.maxConcurrency)
      .reduce((sum, q) => sum + (q.maxConcurrency || 0), 0);
    const utilization =
      globalCapacity > 0
        ? Math.round((globalRunning / globalCapacity) * 100)
        : 0;

    return {
      globalQueues,
      dagBasedQueues,
      activeQueues,
      totalRunning,
      totalQueued,
      totalActive,
      utilization,
    };
  }, [data?.queues]);

  // Handle DAG run click
  const handleDAGRunClick = React.useCallback(
    (dagRun: components['schemas']['DAGRunSummary']) => {
      setModalDAGRun({
        name: dagRun.name,
        dagRunId: dagRun.dagRunId,
      });
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
      {/* Header with search and refresh */}
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
            {filteredQueues.length !== data?.queues?.length && (
              <span className="text-xs text-muted-foreground">
                ({filteredQueues.length} of {data?.queues?.length})
              </span>
            )}
            <RefreshButton onRefresh={handleRefresh} />
          </div>
      </div>

      {/* Metrics */}
      <QueueMetrics metrics={metrics} isLoading={isLoading} />

      {/* Queue List */}
      <div className="flex-1 min-h-0 overflow-auto">
        <QueueList
          queues={filteredQueues}
          isLoading={isLoading && !data}
          onDAGRunClick={handleDAGRunClick}
          onQueueCleared={handleRefresh}
        />
      </div>

      {/* DAG Run Details Modal */}
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
