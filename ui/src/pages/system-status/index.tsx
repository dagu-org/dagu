import { Activity, Calendar, RefreshCw, Server } from 'lucide-react';
import React from 'react';
import type { components } from '../../api/v1/schema';
import { Button } from '../../components/ui/button';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import WorkersSummary from '../../features/dashboard/components/WorkersSummary';
import PathsCard from '../../features/system-status/components/PathsCard';
import ResourceChart from '../../features/system-status/components/ResourceChart';
import ServiceCard from '../../features/system-status/components/ServiceCard';
import TunnelStatusCard from '../../features/system-status/components/TunnelStatusCard';
import { useQuery } from '../../hooks/api';
import { cn } from '../../lib/utils';

type SchedulerInstance = components['schemas']['SchedulerInstance'];
type CoordinatorInstance = components['schemas']['CoordinatorInstance'];
type TunnelStatusResponse = components['schemas']['TunnelStatusResponse'];

/**
 * Render the System Status view showing service health, resource usage charts, and refresh controls.
 *
 * Displays Scheduler and Coordinator service cards, four resource usage charts (CPU, Memory, Disk, Load Average),
 * and controls for toggling auto-refresh and triggering a manual refresh. Data is fetched for the currently
 * selected remote node and the "last updated" timestamp reflects the most recent automatic or manual refresh.
 *
 * @returns The rendered System Status UI containing service cards, resource charts, and refresh controls.
 */
function SystemStatus() {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);
  const [lastUpdateTime, setLastUpdateTime] = React.useState<Date>(new Date());

  React.useEffect(() => {
    appBarContext.setTitle('System Status');
  }, [appBarContext]);

  // Fetch all data with remoteNode support and auto-refresh
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
      refreshInterval: autoRefresh ? 5000 : 0,
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
      refreshInterval: autoRefresh ? 5000 : 0,
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
      refreshInterval: autoRefresh ? 5000 : 0,
    }
  );

  const {
    data: workersData,
    error: workersError,
    mutate: mutateWorkers,
  } = useQuery(
    '/workers',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: autoRefresh ? 1000 : 0,
    }
  );

  const {
    data: tunnelData,
    error: tunnelError,
    mutate: mutateTunnel,
  } = useQuery(
    '/services/tunnel',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      refreshInterval: autoRefresh ? 5000 : 0,
    }
  );

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await Promise.all([
        mutateResource(),
        mutateScheduler(),
        mutateCoordinator(),
        mutateWorkers(),
        mutateTunnel(),
      ]);
      setLastUpdateTime(new Date());
    } finally {
      setIsRefreshing(false);
    }
  };

  // Update timestamp when data changes from auto-refresh
  React.useEffect(() => {
    if (resourceData) {
      setLastUpdateTime(new Date());
    }
  }, [resourceData]);

  return (
    <div className="flex flex-col gap-4 max-w-7xl">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">System Status</h1>
          <p className="text-sm text-muted-foreground">
            Monitor and manage Boltbase services and system health
          </p>
        </div>
        <div className="flex items-center gap-2">
          <PathsCard />
          <Button
            onClick={() => setAutoRefresh(!autoRefresh)}
            aria-label={`Auto-refresh ${autoRefresh ? 'enabled' : 'disabled'}`}
            title={`Toggle auto-refresh (currently ${autoRefresh ? 'ON' : 'OFF'})`}
          >
            <Activity
              className={cn('h-4 w-4', autoRefresh && 'text-success')}
            />
            Auto: {autoRefresh ? 'ON' : 'OFF'}
          </Button>
          <Button
            size="icon"
            onClick={handleRefresh}
            disabled={isRefreshing}
            aria-label="Refresh system status"
            title="Refresh system status"
          >
            <RefreshCw
              className={cn('h-4 w-4', isRefreshing && 'animate-spin')}
            />
          </Button>
        </div>
      </div>

      {/* Services */}
      <div className="flex flex-col gap-4">
        {/* Scheduler Service */}
        <ServiceCard
          title="Scheduler Service"
          instances={
            schedulerData?.schedulers?.map((s: SchedulerInstance) => ({
              instanceId: s.instanceId,
              host: s.host,
              status: s.status,
              startedAt: s.startedAt,
            })) || []
          }
          icon={<Calendar className="h-4 w-4" />}
          isLoading={!schedulerData && !schedulerError}
          error={schedulerError ? String(schedulerError) : undefined}
        />

        {/* Coordinator Service */}
        <ServiceCard
          title="Coordinator Service"
          instances={
            coordinatorData?.coordinators?.map((c: CoordinatorInstance) => ({
              instanceId: c.instanceId,
              host: c.host,
              port: c.port,
              status: c.status,
              startedAt: c.startedAt,
            })) || []
          }
          icon={<Server className="h-4 w-4" />}
          isLoading={!coordinatorData && !coordinatorError}
          error={coordinatorError ? String(coordinatorError) : undefined}
        />

        {/* Tunnel Service */}
        <TunnelStatusCard
          data={tunnelData as TunnelStatusResponse}
          isLoading={!tunnelData && !tunnelError}
          error={tunnelError ? String(tunnelError) : undefined}
        />
      </div>

      {/* Workers Status */}
      <h2 className="text-xl font-semibold mt-8 mb-4">Workers</h2>
      <div className="card-obsidian" style={{ minHeight: '200px' }}>
        <WorkersSummary
          workers={workersData?.workers || []}
          isLoading={!workersData && !workersError}
          errors={workersData?.errors}
        />
      </div>

      {/* Resource Usage */}
      <h2 className="text-xl font-semibold mt-8 mb-4">Resource Usage</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <ResourceChart
          title="CPU Usage"
          data={resourceData?.cpu}
          color="#73BF69"
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
        <ResourceChart
          title="Memory Usage"
          data={resourceData?.memory}
          color="#73BF69"
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
          totalBytes={resourceData?.memoryTotalBytes}
          usedBytes={resourceData?.memoryUsedBytes}
        />
        <ResourceChart
          title="Disk Usage"
          data={resourceData?.disk}
          color="#73BF69"
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
          totalBytes={resourceData?.diskTotalBytes}
          usedBytes={resourceData?.diskUsedBytes}
        />
        <ResourceChart
          title="Load Average"
          data={resourceData?.load}
          color="#73BF69"
          unit=""
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
      </div>

      {/* Footer */}
      <div className="text-xs text-muted-foreground text-center space-y-1 mb-4">
        <div>
          Last updated: {lastUpdateTime.toLocaleTimeString()}
          {autoRefresh && ' • Refreshing every 5 seconds'}
        </div>
        <div>Boltbase v{config.version}</div>
      </div>
    </div>
  );
}

export default SystemStatus;
