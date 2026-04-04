import { Clock } from 'lucide-react';
import React from 'react';
import { cn } from '../../../lib/utils';

interface ServiceInstance {
  instanceId: string;
  host: string;
  status: 'active' | 'inactive' | 'unknown';
  startedAt: string;
  port?: number;
  automataController?: {
    state: 'ready' | 'disabled' | 'unavailable' | 'unknown';
    message?: string;
  };
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
  const activeCount = instances.filter((i) => i.status === 'active').length;

  const getUptime = (startedAt: string): string => {
    const start = new Date(startedAt);
    const now = new Date();
    const diff = now.getTime() - start.getTime();

    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  };

  const getStatusColor = (status: string) => {
    if (status === 'active') return 'bg-success';
    if (status === 'inactive') return 'bg-warning';
    return 'bg-muted-foreground';
  };

  return (
    <div className="border rounded-lg bg-card">
      {/* Header */}
      <div className="flex items-center gap-2 px-3 py-2 border-b">
        <div className="text-muted-foreground">{icon}</div>
        <h3 className="text-sm font-medium">{title}</h3>
        <span className="text-xs text-muted-foreground ml-auto">
          {activeCount}/{instances.length} active
        </span>
      </div>

      {/* Instances List */}
      <div className="divide-y">
        {isLoading && instances.length === 0 && (
          <div className="px-3 py-2 text-xs text-muted-foreground">Loading...</div>
        )}

        {error && (
          <div className="px-3 py-2 text-xs text-error">{error}</div>
        )}

        {!error && instances.length === 0 && !isLoading && (
          <div className="px-3 py-2 text-xs text-muted-foreground">No instances</div>
        )}

        {instances.map((instance) => (
          <div key={instance.instanceId} className="flex items-center gap-3 px-3 py-1.5 text-xs">
            {/* Status indicator */}
            <div className="relative flex-shrink-0">
              <div className={cn('w-1.5 h-1.5 rounded-full', getStatusColor(instance.status))} />
              {instance.status === 'active' && (
                <div className={cn('absolute inset-0 rounded-full animate-ping opacity-75', getStatusColor(instance.status))} />
              )}
            </div>

            {/* Host:Port */}
            <span className="font-mono text-muted-foreground">
              {instance.host}{instance.port ? `:${instance.port}` : ''}
            </span>

            <div className="ml-auto flex items-center gap-3">
              {instance.automataController ? (
                <span
                  className={cn(
                    'rounded-full px-2 py-0.5 text-[11px] font-medium',
                    instance.automataController.state === 'ready'
                      ? 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200'
                      : instance.automataController.state === 'disabled'
                        ? 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-200'
                        : instance.automataController.state === 'unavailable'
                          ? 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200'
                          : 'bg-muted text-muted-foreground'
                  )}
                  title={instance.automataController.message}
                >
                  automata {instance.automataController.state}
                </span>
              ) : null}

              {/* Uptime */}
              {instance.status === 'active' && (
                <span className="flex items-center gap-1 text-muted-foreground">
                  <Clock className="h-3 w-3" />
                  {getUptime(instance.startedAt)}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default ServiceCard;
