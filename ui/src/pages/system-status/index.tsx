import { Activity, Calendar, RefreshCw, Server } from 'lucide-react';
import React from 'react';
import type { components } from '../../api/v2/schema';
import { Button } from '../../components/ui/button';
import { AppBarContext } from '../../contexts/AppBarContext';
import PathsCard from '../../features/system-status/components/PathsCard';
import ResourceChart from '../../features/system-status/components/ResourceChart';
import ServiceCard from '../../features/system-status/components/ServiceCard';
import { useQuery } from '../../hooks/api';
import { cn } from '../../lib/utils';

type SchedulerInstance = components['schemas']['SchedulerInstance'];
type CoordinatorInstance = components['schemas']['CoordinatorInstance'];

function SystemStatus() {
  const appBarContext = React.useContext(AppBarContext);
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);

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

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await Promise.all([mutateResource(), mutateScheduler(), mutateCoordinator()]);
    setIsRefreshing(false);
  };

  return (
    <div className="flex flex-col gap-4 p-4 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">System Status</h1>
          <p className="text-sm text-muted-foreground">
            Monitor and manage Dagu services and system health
          </p>
        </div>
        <div className="flex items-center gap-2">
          <PathsCard />
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={cn(
              'h-7 px-2',
              autoRefresh && 'bg-green-50 dark:bg-green-950 border-green-500'
            )}
          >
            <Activity
              className={cn('h-3 w-3 mr-1', autoRefresh && 'text-green-500')}
            />
            <span className="text-xs">Auto: {autoRefresh ? 'ON' : 'OFF'}</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={isRefreshing}
            className="h-7 px-2"
          >
            <RefreshCw
              className={cn('h-3 w-3', isRefreshing && 'animate-spin')}
            />
          </Button>
        </div>
      </div>

      {/* Services Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
      </div>

      {/* Resource Usage */}
      <h2 className="text-xl font-semibold mt-8 mb-4">Resource Usage</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <ResourceChart
          title="CPU Usage"
          data={resourceData?.cpu}
          color="#3b82f6" // blue-500
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
        <ResourceChart
          title="Memory Usage"
          data={resourceData?.memory}
          color="#8b5cf6" // violet-500
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
        <ResourceChart
          title="Disk Usage"
          data={resourceData?.disk}
          color="#10b981" // emerald-500
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
        <ResourceChart
          title="Load Average"
          data={resourceData?.load}
          color="#f59e0b" // amber-500
          unit=""
          isLoading={!resourceData && !resourceError}
          error={resourceError ? String(resourceError) : undefined}
        />
      </div>

      {/* Last Update */}
      <div className="text-xs text-muted-foreground text-center">
        Last updated: {new Date().toLocaleTimeString()}
        {autoRefresh && ' • Refreshing every 5 seconds'}
      </div>
    </div>
  );
}

export default SystemStatus;
