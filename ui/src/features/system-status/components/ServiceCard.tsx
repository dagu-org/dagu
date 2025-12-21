import { ChevronDown, ChevronUp, Clock, Server } from 'lucide-react';
import React from 'react';
import { cn } from '../../../lib/utils';

interface ServiceInstance {
  instanceId: string;
  host: string;
  status: 'active' | 'inactive' | 'unknown';
  startedAt: string;
  port?: number;
}

interface ServiceCardProps {
  title: string;
  instances: ServiceInstance[];
  icon: React.ReactNode;
  isLoading?: boolean;
  error?: string;
}

function ServiceCard({
  title,
  instances,
  icon,
  isLoading,
  error,
}: ServiceCardProps) {
  const [isExpanded, setIsExpanded] = React.useState(false);

  // Calculate overall status
  const activeCount = instances.filter((i) => i.status === 'active').length;
  const hasActive = activeCount > 0;
  const allActive = activeCount === instances.length && instances.length > 0;

  const getStatusColor = () => {
    if (error) return 'text-error';
    if (!hasActive) return 'text-warning';
    if (allActive) return 'text-success';
    return 'text-success';
  };

  const getUptime = (startedAt: string): string => {
    const start = new Date(startedAt);
    const now = new Date();
    const diff = now.getTime() - start.getTime();

    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

    if (days > 0) {
      return `${days}d ${hours}h`;
    } else if (hours > 0) {
      return `${hours}h ${minutes}m`;
    } else {
      return `${minutes}m`;
    }
  };

  return (
    <div className="border rounded-lg bg-card">
      <div className="p-3">
        {/* Header */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <div className="text-muted-foreground">{icon}</div>
            <h3 className="text-sm font-semibold">{title}</h3>
          </div>
          {instances.length > 1 && (
            <button
              onClick={() => setIsExpanded(!isExpanded)}
              className="p-1 hover:bg-muted rounded transition-colors"
            >
              {isExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </button>
          )}
        </div>

        {/* Status Summary */}
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1.5">
            <div className="relative">
              <div
                className={cn(
                  'w-2 h-2 rounded-full transition-colors',
                  getStatusColor()
                )}
              />
              {hasActive && !error && (
                <div
                  className={cn(
                    'absolute inset-0 rounded-full animate-ping opacity-75',
                    getStatusColor()
                  )}
                />
              )}
            </div>
            <span className={cn('font-medium', getStatusColor())}>
              {error
                ? 'Error'
                : activeCount > 0
                  ? `${activeCount} Active`
                  : 'Inactive'}
            </span>
          </div>

          {instances.length > 0 && (
            <div className="text-muted-foreground">
              {instances.length} instance{instances.length !== 1 ? 's' : ''}
            </div>
          )}
        </div>

        {/* Primary Instance Info (always visible) */}
        {instances.length > 0 && instances[0] && !error && (
          <div className="mt-2 pt-2 border-t space-y-1">
            <div className="flex items-center gap-2 text-xs">
              <Server className="h-3 w-3 text-muted-foreground" />
              <span className="text-muted-foreground">
                {instances[0].host}
                {instances[0].port ? `:${instances[0].port}` : ''}
              </span>
            </div>
            {instances[0].status === 'active' && (
              <div className="flex items-center gap-2 text-xs">
                <Clock className="h-3 w-3 text-muted-foreground" />
                <span className="text-muted-foreground">
                  Uptime: {getUptime(instances[0].startedAt)}
                </span>
              </div>
            )}
          </div>
        )}

        {/* Expanded Instance List */}
        {isExpanded && instances.length > 1 && (
          <div className="mt-2 pt-2 border-t space-y-2">
            {instances.slice(1).map((instance) => (
              <div
                key={instance.instanceId}
                className="pl-3 border-l-2 border-muted"
              >
                <div className="flex items-center gap-2 text-xs">
                  <div
                    className={cn(
                      'w-1.5 h-1.5 rounded-full',
                      instance.status === 'active'
                        ? 'bg-success'
                        : instance.status === 'inactive'
                          ? 'bg-warning'
                          : 'bg-muted-foreground'
                    )}
                  />
                  <span className="text-muted-foreground">
                    {instance.host}
                    {instance.port ? `:${instance.port}` : ''}
                  </span>
                </div>
                {instance.status === 'active' && (
                  <div className="flex items-center gap-2 text-xs mt-0.5">
                    <Clock className="h-3 w-3 text-muted-foreground ml-3.5" />
                    <span className="text-muted-foreground">
                      {getUptime(instance.startedAt)}
                    </span>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Error Message */}
        {error && <div className="mt-2 text-xs text-error">{error}</div>}

        {/* Loading State */}
        {isLoading && (
          <div className="mt-2 text-xs text-muted-foreground">Loading...</div>
        )}
      </div>
    </div>
  );
}

export default ServiceCard;
