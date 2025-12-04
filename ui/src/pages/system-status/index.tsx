import React from 'react';
import { Calendar, Server, RefreshCw, Activity } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useQuery } from '../../hooks/api';
import ServiceCard from '../../features/system-status/components/ServiceCard';
import PathsCard from '../../features/system-status/components/PathsCard';
import { cn } from '../../lib/utils';
import type { components } from '../../api/v2/schema';

type SchedulerInstance = components['schemas']['SchedulerInstance'];
type CoordinatorInstance = components['schemas']['CoordinatorInstance'];

function SystemStatus() {
  const appBarContext = React.useContext(AppBarContext);
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);

  React.useEffect(() => {
    appBarContext.setTitle('System Status');
  }, [appBarContext]);

  // Fetch all data with remoteNode support
  const { data: schedulerData, error: schedulerError } = useQuery('/services/scheduler', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
    },
  });

  const { data: coordinatorData, error: coordinatorError } = useQuery('/services/coordinator', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
    },
  });

  const handleRefresh = async () => {
    setIsRefreshing(true);
    // The SWR hooks will automatically revalidate when we change a dependency
    setTimeout(() => setIsRefreshing(false), 500);
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
              "h-7 px-2",
              autoRefresh && "bg-green-50 dark:bg-green-950 border-green-500"
            )}
          >
            <Activity className={cn(
              "h-3 w-3 mr-1",
              autoRefresh && "text-green-500"
            )} />
            <span className="text-xs">
              Auto: {autoRefresh ? 'ON' : 'OFF'}
            </span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={isRefreshing}
            className="h-7 px-2"
          >
            <RefreshCw className={cn(
              "h-3 w-3",
              isRefreshing && "animate-spin"
            )} />
          </Button>
        </div>
      </div>

      {/* Services Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Scheduler Service */}
        <ServiceCard
          title="Scheduler Service"
          instances={schedulerData?.schedulers?.map((s: SchedulerInstance) => ({
            instanceId: s.instanceId,
            host: s.host,
            status: s.status,
            startedAt: s.startedAt,
          })) || []}
          icon={<Calendar className="h-4 w-4" />}
          isLoading={!schedulerData && !schedulerError}
          error={schedulerError ? String(schedulerError) : undefined}
        />

        {/* Coordinator Service */}
        <ServiceCard
          title="Coordinator Service"
          instances={coordinatorData?.coordinators?.map((c: CoordinatorInstance) => ({
            instanceId: c.instanceId,
            host: c.host,
            port: c.port,
            status: c.status,
            startedAt: c.startedAt,
          })) || []}
          icon={<Server className="h-4 w-4" />}
          isLoading={!coordinatorData && !coordinatorError}
          error={coordinatorError ? String(coordinatorError) : undefined}
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
