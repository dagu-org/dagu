import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { RefreshButton } from '@/components/ui/refresh-button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Filter } from 'lucide-react';
import React from 'react';
import type { components } from '../api/v2/schema';
import { Status } from '../api/v2/schema';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { useSearchState } from '../contexts/SearchStateContext';
import { DAGRunDetailsModal } from '../features/dag-runs/components/dag-run-details';
import DashboardTimeChart from '../features/dashboard/components/DashboardTimechart';
import MiniResourceChart from '../features/dashboard/components/MiniResourceChart';
import WorkersSummary from '../features/dashboard/components/WorkersSummary';
import PathsCard from '../features/system-status/components/PathsCard';
import { useQuery } from '../hooks/api';
import dayjs from '../lib/dayjs';
import Title from '../ui/Title';

type DAGRunSummary = components['schemas']['DAGRunSummary'];
type SchedulerInstance = components['schemas']['SchedulerInstance'];
type CoordinatorInstance = components['schemas']['CoordinatorInstance'];

type Metrics = Record<Status, number>;

const initializeMetrics = (): Metrics => {
  const initialMetrics: Partial<Metrics> = {};
  const relevantStatuses = [
    Status.Success,
    Status.Failed,
    Status.Running,
    Status.Aborted,
    Status.Queued,
    Status.NotStarted,
    Status.PartialSuccess,
  ];
  relevantStatuses.forEach((status: Status) => {
    initialMetrics[status] = 0;
  });
  return initialMetrics as Metrics;
};

function Dashboard(): React.ReactElement | null {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  // State for DAG run modal
  const [modalDAGRun, setModalDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);

  type DashboardFilters = {
    selectedDAGRun: string;
    dateRange: {
      startDate: number;
      endDate: number | undefined;
    };
  };

  const areFiltersEqual = (a: DashboardFilters, b: DashboardFilters) =>
    a.selectedDAGRun === b.selectedDAGRun &&
    a.dateRange.startDate === b.dateRange.startDate &&
    (a.dateRange.endDate ?? null) === (b.dateRange.endDate ?? null);

  const getDefaultDateRange = React.useCallback(() => {
    const now = dayjs();
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');
    return {
      startDate: startOfDay.unix(),
      endDate: undefined,
    };
  }, [config.tzOffsetInSec]);

  const defaultFilters = React.useMemo<DashboardFilters>(
    () => ({
      selectedDAGRun: 'all',
      dateRange: getDefaultDateRange(),
    }),
    [getDefaultDateRange]
  );

  const [selectedDAGRun, setSelectedDAGRun] = React.useState<string>(
    defaultFilters.selectedDAGRun
  );
  const [dateRange, setDateRange] = React.useState<{
    startDate: number;
    endDate: number | undefined;
  }>(defaultFilters.dateRange);

  const currentFilters = React.useMemo<DashboardFilters>(
    () => ({
      selectedDAGRun,
      dateRange,
    }),
    [selectedDAGRun, dateRange]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef = React.useRef<DashboardFilters | null>(null);

  React.useEffect(() => {
    const stored = searchState.readState<DashboardFilters>(
      'dashboard',
      remoteKey
    );
    const base = defaultFilters;
    const next = stored
      ? {
          selectedDAGRun: stored.selectedDAGRun || base.selectedDAGRun,
          dateRange: {
            startDate: stored.dateRange?.startDate ?? base.dateRange.startDate,
            endDate:
              stored.dateRange?.endDate === undefined
                ? base.dateRange.endDate
                : stored.dateRange.endDate,
          },
        }
      : base;

    const current = currentFiltersRef.current;
    if (current && areFiltersEqual(current, next)) {
      if (!stored) {
        searchState.writeState('dashboard', remoteKey, next);
      }
      lastPersistedFiltersRef.current = next;
      return;
    }

    setSelectedDAGRun(next.selectedDAGRun);
    setDateRange(next.dateRange);
    lastPersistedFiltersRef.current = next;
    searchState.writeState('dashboard', remoteKey, next);
  }, [defaultFilters, remoteKey, searchState]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areFiltersEqual(persisted, currentFilters)) {
      return;
    }
    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('dashboard', remoteKey, currentFilters);
  }, [currentFilters, remoteKey, searchState]);

  const handleDateChange = (startTimestamp: number, endTimestamp: number) => {
    setDateRange({
      startDate: startTimestamp,
      endDate: endTimestamp,
    });
  };

  // DAG runs data
  const { data, error, isLoading, mutate } = useQuery('/dag-runs', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
        fromDate: dateRange.startDate,
        toDate: dateRange.endDate,
        name: selectedDAGRun !== 'all' ? selectedDAGRun : undefined,
      },
    },
    refreshInterval: 5000,
  });

  // System status data
  const {
    data: schedulerData,
    error: schedulerError,
    mutate: mutateScheduler,
  } = useQuery(
    '/services/scheduler',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: 5000,
    }
  );

  const {
    data: coordinatorData,
    error: coordinatorError,
    mutate: mutateCoordinator,
  } = useQuery(
    '/services/coordinator',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: 5000,
    }
  );

  const {
    data: resourceData,
    error: resourceError,
    mutate: mutateResource,
  } = useQuery(
    '/services/resources/history',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: 5000,
    }
  );

  // Workers data
  const {
    data: workersData,
    error: workersError,
    isLoading: workersLoading,
    mutate: mutateWorkers,
  } = useQuery('/workers', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
    },
    refreshInterval: 1000,
  });

  const handleRefreshAll = async () => {
    await Promise.all([
      mutate(),
      mutateResource(),
      mutateScheduler(),
      mutateCoordinator(),
      mutateWorkers(),
    ]);
  };

  // Handle task click from workers
  const handleTaskClick = React.useCallback(
    (task: components['schemas']['RunningTask']) => {
      if (task.parentDagRunName && task.parentDagRunId) {
        const searchParams = new URLSearchParams();
        searchParams.set('subDAGRunId', task.dagRunId);
        searchParams.set('dagRunId', task.parentDagRunId);
        searchParams.set('dagRunName', task.parentDagRunName);
        window.history.pushState(
          {},
          '',
          `${window.location.pathname}?${searchParams.toString()}`
        );

        setModalDAGRun({
          name: task.parentDagRunName,
          dagRunId: task.parentDagRunId,
        });
      } else {
        setModalDAGRun({
          name: task.dagName,
          dagRunId: task.dagRunId,
        });
      }
    },
    []
  );

  const dagRunsList: DAGRunSummary[] = data?.dagRuns || [];

  const uniqueDAGRunNames = React.useMemo(() => {
    const names = new Set<string>();
    if (data && data.dagRuns) {
      data.dagRuns.forEach((dagRun) => {
        if (dagRun.name) {
          names.add(dagRun.name);
        }
      });
    }
    return Array.from(names).sort();
  }, [data]);

  const handleDAGRunChange = (value: string) => {
    setSelectedDAGRun(value);
  };

  React.useEffect(() => {
    if (appBarContext) {
      appBarContext.setTitle('Dashboard');
    }
  }, [appBarContext]);

  if (error) {
    const errorData = error as components['schemas']['Error'];
    const errorMessage =
      errorData?.message || 'Unknown error loading dashboard';
    return <div className="p-4 text-error">Error: {errorMessage}</div>;
  }

  const metrics = initializeMetrics();
  const totalDAGRuns = dagRunsList.length;

  dagRunsList.forEach((dagRun) => {
    if (
      dagRun &&
      Object.prototype.hasOwnProperty.call(metrics, dagRun.status)
    ) {
      const statusKey = dagRun.status as Status;
      metrics[statusKey]! += 1;
    }
  });

  // Compute health indicators
  const hasFailures = metrics[Status.Failed] > 0;
  const hasRunning = metrics[Status.Running] > 0;

  // Service health
  const schedulerActive = schedulerData?.schedulers?.some((s: SchedulerInstance) => s.status === 'active');
  const coordinatorActive = coordinatorData?.coordinators?.some((c: CoordinatorInstance) => c.status === 'active');
  const servicesHealthy = schedulerActive && coordinatorActive;

  return (
    <div className="flex flex-col w-full h-full overflow-hidden">
      <Title>Dashboard</Title>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-h-0 gap-3 p-1">

        {/* Toolbar - Top */}
        <div className="flex flex-wrap items-center gap-2 flex-shrink-0">
          <Select
            value={selectedDAGRun}
            onValueChange={handleDAGRunChange}
            disabled={isLoading}
          >
            <SelectTrigger className="h-9 w-[140px]">
              <Filter className="h-4 w-4 mr-1.5 text-muted-foreground" />
              <SelectValue placeholder={isLoading ? 'Loading...' : 'All DAGs'} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All DAGs</SelectItem>
              {uniqueDAGRunNames.map((name) => (
                <SelectItem key={name} value={name}>{name}</SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Input
            type="date"
            value={dayjs.unix(dateRange.startDate).format('YYYY-MM-DD')}
            onChange={(e) => {
              const newDate = e.target.value;
              if (!newDate) return;
              const date = dayjs(newDate);
              if (!date.isValid()) return;
              const startOfDay = config.tzOffsetInSec !== undefined
                ? date.utcOffset(config.tzOffsetInSec / 60).startOf('day')
                : date.startOf('day');
              const endOfDay = config.tzOffsetInSec !== undefined
                ? date.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                : date.endOf('day');
              handleDateChange(startOfDay.unix(), endOfDay.unix());
            }}
            className="h-9 w-[140px]"
          />
          <Button
            variant="outline"
            onClick={() => {
              const now = dayjs();
              const startOfDay = config.tzOffsetInSec !== undefined
                ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
                : now.startOf('day');
              const endOfDay = config.tzOffsetInSec !== undefined
                ? now.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                : now.endOf('day');
              handleDateChange(startOfDay.unix(), endOfDay.unix());
            }}
          >
            Today
          </Button>

          <div className="flex-1" />

          <PathsCard />
          <RefreshButton onRefresh={handleRefreshAll} />
        </div>

        {/* Stats Row */}
        <div className="flex items-baseline gap-6 text-sm text-muted-foreground flex-shrink-0">
          <div className="flex items-baseline gap-1.5">
            <span className="text-xl font-light tabular-nums text-foreground">{totalDAGRuns}</span>
            <span className="text-xs">runs</span>
          </div>
          <div className="flex items-baseline gap-1.5">
            <span className="text-xl font-light tabular-nums text-foreground">{metrics[Status.Success]}</span>
            <span className="text-xs">succeeded</span>
          </div>
          <div className="flex items-baseline gap-1.5">
            <span className={`text-xl font-light tabular-nums ${hasFailures ? 'text-foreground' : 'text-muted-foreground/50'}`}>{metrics[Status.Failed]}</span>
            <span className="text-xs">failed</span>
          </div>
          {hasRunning && (
            <div className="flex items-baseline gap-1.5">
              <span className="text-xl font-light tabular-nums text-foreground">{metrics[Status.Running]}</span>
              <span className="text-xs">active</span>
              <span className="relative flex h-1.5 w-1.5 self-center ml-0.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-foreground/40 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-foreground/70" />
              </span>
            </div>
          )}
          {metrics[Status.Queued] > 0 && (
            <div className="flex items-baseline gap-1.5">
              <span className="text-xl font-light tabular-nums text-foreground">{metrics[Status.Queued]}</span>
              <span className="text-xs">queued</span>
            </div>
          )}
          <div className="flex-1" />
          <div className="flex items-center gap-1.5 text-xs">
            <span className={`h-1.5 w-1.5 rounded-full ${servicesHealthy ? 'bg-foreground/50' : 'bg-foreground'}`} />
            <span>{servicesHealthy ? 'Healthy' : 'Degraded'}</span>
          </div>
        </div>

        {/* Timeline Visualization - Hero */}
        <div className="flex-[2] min-h-[250px] rounded-xl border border-border bg-surface overflow-hidden">
          <DashboardTimeChart
            data={dagRunsList}
            selectedDate={{
              startTimestamp: dateRange.startDate,
              endTimestamp: dateRange.endDate,
            }}
          />
        </div>

        {/* Live Workers */}
        <div className="flex-1 min-h-[120px] rounded-xl border border-border bg-surface overflow-hidden">
          <WorkersSummary
            workers={workersData?.workers || []}
            isLoading={workersLoading && !workersData}
            errors={workersData?.errors}
            onTaskClick={handleTaskClick}
          />
        </div>

        {/* System Resources - Full Width Row */}
        <div className="h-24 flex-shrink-0 rounded-xl border border-border bg-surface p-3">
          <div className="grid grid-cols-4 gap-4 h-full">
            <MiniResourceChart
              title="CPU"
              data={resourceData?.cpu}
              color="#c4956a"
              isLoading={!resourceData && !resourceError}
              error={resourceError ? String(resourceError) : undefined}
            />
            <MiniResourceChart
              title="Memory"
              data={resourceData?.memory}
              color="#8a9fc4"
              isLoading={!resourceData && !resourceError}
              error={resourceError ? String(resourceError) : undefined}
            />
            <MiniResourceChart
              title="Disk"
              data={resourceData?.disk}
              color="#7da87d"
              isLoading={!resourceData && !resourceError}
              error={resourceError ? String(resourceError) : undefined}
            />
            <MiniResourceChart
              title="Load"
              data={resourceData?.load}
              color="#d4a574"
              unit=""
              isLoading={!resourceData && !resourceError}
              error={resourceError ? String(resourceError) : undefined}
            />
          </div>
        </div>
      </div>

      {/* DAG Run Details Modal */}
      {modalDAGRun && (
        <DAGRunDetailsModal
          name={modalDAGRun.name}
          dagRunId={modalDAGRun.dagRunId}
          isOpen={!!modalDAGRun}
          onClose={() => {
            setModalDAGRun(null);
            window.history.pushState({}, '', window.location.pathname);
          }}
        />
      )}
    </div>
  );
}

export default Dashboard;
