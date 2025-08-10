import React from 'react';
import { Heart, Clock, Package, Activity, AlertCircle } from 'lucide-react';
import { cn } from '../../../lib/utils';
import type { components } from '../../../api/v2/schema';

type HealthResponse = components['schemas']['HealthResponse'];

interface SystemOverviewProps {
  health?: HealthResponse;
  totalDAGs?: number;
  activeRuns?: number;
  recentErrors?: number;
  isLoading?: boolean;
  error?: string;
}

function SystemOverview({ 
  health, 
  totalDAGs = 0, 
  activeRuns = 0, 
  recentErrors = 0,
  isLoading, 
  error 
}: SystemOverviewProps) {
  
  const formatUptime = (seconds: number): string => {
    const days = Math.floor(seconds / (60 * 60 * 24));
    const hours = Math.floor((seconds % (60 * 60 * 24)) / (60 * 60));
    const minutes = Math.floor((seconds % (60 * 60)) / 60);
    
    if (days > 0) {
      return `${days}d ${hours}h ${minutes}m`;
    } else if (hours > 0) {
      return `${hours}h ${minutes}m`;
    } else {
      return `${minutes}m`;
    }
  };
  
  const getHealthColor = () => {
    if (error || !health) return 'text-red-500';
    return health.status === 'healthy' ? 'text-green-500' : 'text-yellow-500';
  };
  
  return (
    <div className="border rounded-lg bg-card p-4">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-base font-semibold">System Overview</h2>
        <div className="flex items-center gap-2">
          <div className="relative">
            <div className={cn(
              "w-2 h-2 rounded-full transition-colors",
              getHealthColor()
            )} />
            {health?.status === 'healthy' && !error && (
              <div className={cn(
                "absolute inset-0 rounded-full animate-ping opacity-75",
                getHealthColor()
              )} />
            )}
          </div>
          <span className={cn("text-xs font-medium", getHealthColor())}>
            {error ? 'Error' : health?.status === 'healthy' ? 'Healthy' : 'Degraded'}
          </span>
        </div>
      </div>
      
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {/* Server Health */}
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Heart className="h-3 w-3" />
            <span>Server Status</span>
          </div>
          <div className="text-sm font-medium">
            {health?.status || 'Unknown'}
          </div>
          <div className="text-xs text-muted-foreground">
            v{health?.version || 'Unknown'}
          </div>
        </div>
        
        {/* Uptime */}
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Clock className="h-3 w-3" />
            <span>Uptime</span>
          </div>
          <div className="text-sm font-medium">
            {health?.uptime ? formatUptime(health.uptime) : 'N/A'}
          </div>
          <div className="text-xs text-muted-foreground">
            Since startup
          </div>
        </div>
        
        {/* Total DAGs */}
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Package className="h-3 w-3" />
            <span>Total DAGs</span>
          </div>
          <div className="text-sm font-medium">
            {totalDAGs}
          </div>
          <div className="text-xs text-muted-foreground">
            Definitions
          </div>
        </div>
        
        {/* Active Runs */}
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Activity className="h-3 w-3" />
            <span>Active Runs</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="text-sm font-medium">
              {activeRuns}
            </div>
            {recentErrors > 0 && (
              <div className="flex items-center gap-1 text-xs text-red-500">
                <AlertCircle className="h-3 w-3" />
                <span>{recentErrors}</span>
              </div>
            )}
          </div>
          <div className="text-xs text-muted-foreground">
            Running now
          </div>
        </div>
      </div>
      
      {error && (
        <div className="mt-3 pt-3 border-t text-xs text-red-500">
          {error}
        </div>
      )}
    </div>
  );
}

export default SystemOverview;