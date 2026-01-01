import React from 'react';

interface QueueMetricsProps {
  metrics: {
    globalQueues: number;
    dagBasedQueues: number;
    activeQueues: number;
    totalRunning: number;
    totalQueued: number;
    totalActive: number;
    utilization: number;
  };
  isLoading?: boolean;
}

function QueueMetrics({ metrics, isLoading }: QueueMetricsProps) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 sm:gap-x-6 text-sm text-muted-foreground flex-shrink-0">
      <div className="flex items-baseline gap-1">
        <span className="text-lg sm:text-xl font-light tabular-nums text-foreground">
          {isLoading ? '-' : metrics.globalQueues}
        </span>
        <span className="text-xs">global</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-lg sm:text-xl font-light tabular-nums text-foreground">
          {isLoading ? '-' : metrics.dagBasedQueues}
        </span>
        <span className="text-xs">dag-based</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-lg sm:text-xl font-light tabular-nums text-foreground">
          {isLoading ? '-' : metrics.totalRunning}
        </span>
        <span className="text-xs">running</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className={`text-lg sm:text-xl font-light tabular-nums ${metrics.totalQueued > 0 ? 'text-foreground' : 'text-muted-foreground/50'}`}>
          {isLoading ? '-' : metrics.totalQueued}
        </span>
        <span className="text-xs">queued</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-lg sm:text-xl font-light tabular-nums text-foreground">
          {isLoading ? '-' : `${metrics.utilization}%`}
        </span>
        <span className="text-xs">util</span>
      </div>
    </div>
  );
}

export default QueueMetrics;
