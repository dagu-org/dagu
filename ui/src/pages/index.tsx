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
import {
  Calendar,
  CheckCircle,
  Clock,
  Filter,
  ListChecks,
  Loader2,
  Play,
  Server,
  StopCircle,
  XCircle,
} from 'lucide-react';
import React from 'react';
import type { components } from '../api/v2/schema';
import { Status } from '../api/v2/schema';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { useSearchState } from '../contexts/SearchStateContext';
import { DAGRunDetailsModal } from '../features/dag-runs/components/dag-run-details';
import DashboardTimeChart from '../features/dashboard/components/DashboardTimechart';
import MiniResourceChart from '../features/dashboard/components/MiniResourceChart';
import MiniServiceCard from '../features/dashboard/components/MiniServiceCard';
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

  const metricCards = [
    {
      title: 'Total',
      value: totalDAGRuns,
      icon: <ListChecks className="h-3.5 w-3.5 text-muted-foreground" />,
    },
    {
      title: 'Running',
      value: metrics[Status.Running],
      icon: <Play className="h-3.5 w-3.5 text-success" />,
    },
    {
      title: 'Queued',
      value: metrics[Status.Queued],
      icon: <Clock className="h-3.5 w-3.5 text-[purple]" />,
    },
    {
      title: 'Success',
      value: metrics[Status.Success],
      icon: <CheckCircle className="h-3.5 w-3.5 text-success" />,
    },
    {
      title: 'Partial',
      value: metrics[Status.PartialSuccess],
      icon: <CheckCircle className="h-3.5 w-3.5 text-warning" />,
    },
    {
      title: 'Failed',
      value: metrics[Status.Failed],
      icon: <XCircle className="h-3.5 w-3.5 text-error" />,
    },
    {
      title: 'Aborted',
      value: metrics[Status.Aborted],
      icon: <StopCircle className="h-3.5 w-3.5 text-[deeppink]" />,
    },
  ];

  return (
    <div className="flex flex-col gap-2 w-full h-full overflow-auto">
      <Title>Dashboard</Title>
      {/* Header: Filters + Actions + Metrics */}
      <div className="border rounded bg-card flex-shrink-0">
        <div className="flex flex-wrap items-center justify-between gap-2 p-2">
          {/* Left: Filters */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-1.5">
              <Filter className="h-3.5 w-3.5 text-muted-foreground" />
              <Select
                value={selectedDAGRun}
                onValueChange={handleDAGRunChange}
                disabled={isLoading}
              >
                <SelectTrigger className="h-7 w-[160px] text-xs">
                  <SelectValue
                    placeholder={isLoading ? 'Loading...' : 'All DAGs'}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all" className="text-xs">
                    All DAGs
                  </SelectItem>
                  {uniqueDAGRunNames.map((name) => (
                    <SelectItem key={name} value={name} className="text-xs">
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="relative">
                <Input
                  type="date"
                  value={dayjs.unix(dateRange.startDate).format('YYYY-MM-DD')}
                  onChange={(e) => {
                    const newDate = e.target.value;
                    if (!newDate) return;
                    const date = dayjs(newDate);
                    if (!date.isValid()) return;
                    const startOfDay =
                      config.tzOffsetInSec !== undefined
                        ? date
                            .utcOffset(config.tzOffsetInSec / 60)
                            .startOf('day')
                        : date.startOf('day');
                    const endOfDay =
                      config.tzOffsetInSec !== undefined
                        ? date.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                        : date.endOf('day');
                    handleDateChange(startOfDay.unix(), endOfDay.unix());
                  }}
                  className="h-7 w-[130px] text-xs"
                />
                {isLoading && (
                  <Loader2 className="absolute right-2 top-1.5 h-4 w-4 animate-spin text-muted-foreground" />
                )}
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  const now = dayjs();
                  const startOfDay =
                    config.tzOffsetInSec !== undefined
                      ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
                      : now.startOf('day');
                  const endOfDay =
                    config.tzOffsetInSec !== undefined
                      ? now.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                      : now.endOf('day');
                  handleDateChange(startOfDay.unix(), endOfDay.unix());
                }}
                className="h-7 text-xs px-2"
              >
                Today
              </Button>
            </div>
          </div>

          {/* Right: Actions */}
          <div className="flex items-center gap-1.5">
            <PathsCard />
            <RefreshButton onRefresh={handleRefreshAll} />
          </div>
        </div>

        {/* Metrics Row */}
        <div className="flex flex-wrap items-center border-t px-2 py-1.5 gap-x-4 gap-y-1">
          {metricCards.map((card) => (
            <div key={card.title} className="flex items-center gap-1.5">
              {card.icon}
              <span className="text-xs text-muted-foreground">
                {card.title}
              </span>
              <span className="text-sm font-semibold">{card.value}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Timeline Chart - fixed height */}
      <div className="border rounded bg-card flex-shrink-0">
        <div className="h-64">
          <DashboardTimeChart
            data={dagRunsList}
            selectedDate={{
              startTimestamp: dateRange.startDate,
              endTimestamp: dateRange.endDate,
            }}
          />
        </div>
      </div>

      {/* Workers */}
      <div className="border rounded bg-card h-48 flex-shrink-0 flex flex-col min-h-0">
        <WorkersSummary
          workers={workersData?.workers || []}
          isLoading={workersLoading && !workersData}
          errors={workersData?.errors}
          onTaskClick={handleTaskClick}
        />
      </div>

      {/* System Status: Services + Resources */}
      <div className="border rounded bg-card flex-shrink-0">
        <div className="flex items-center gap-2 px-3 py-2 border-b">
          <Server className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">System</span>
        </div>
        <div className="flex flex-wrap items-stretch">
          {/* Services */}
          <div className="flex items-center gap-4 px-3 py-2 border-r">
            <MiniServiceCard
              title="Scheduler"
              instances={
                schedulerData?.schedulers?.map((s: SchedulerInstance) => ({
                  instanceId: s.instanceId,
                  host: s.host,
                  status: s.status,
                  startedAt: s.startedAt,
                })) || []
              }
              icon={<Calendar className="h-3.5 w-3.5" />}
              isLoading={!schedulerData && !schedulerError}
              error={schedulerError ? String(schedulerError) : undefined}
            />
            <MiniServiceCard
              title="Coordinator"
              instances={
                coordinatorData?.coordinators?.map(
                  (c: CoordinatorInstance) => ({
                    instanceId: c.instanceId,
                    host: c.host,
                    port: c.port,
                    status: c.status,
                    startedAt: c.startedAt,
                  })
                ) || []
              }
              icon={<Server className="h-3.5 w-3.5" />}
              isLoading={!coordinatorData && !coordinatorError}
              error={coordinatorError ? String(coordinatorError) : undefined}
            />
          </div>

          {/* Resource Charts */}
          <div className="flex-1 grid grid-cols-2 md:grid-cols-4 gap-3 px-3 py-2">
            <div className="h-12">
              <MiniResourceChart
                title="CPU"
                data={resourceData?.cpu}
                color="#c4956a"
                isLoading={!resourceData && !resourceError}
                error={resourceError ? String(resourceError) : undefined}
              />
            </div>
            <div className="h-12">
              <MiniResourceChart
                title="Memory"
                data={resourceData?.memory}
                color="#8a9fc4"
                isLoading={!resourceData && !resourceError}
                error={resourceError ? String(resourceError) : undefined}
              />
            </div>
            <div className="h-12">
              <MiniResourceChart
                title="Disk"
                data={resourceData?.disk}
                color="#7da87d"
                isLoading={!resourceData && !resourceError}
                error={resourceError ? String(resourceError) : undefined}
              />
            </div>
            <div className="h-12">
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
