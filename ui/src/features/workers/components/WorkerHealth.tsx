import React from 'react';
import { cn } from '../../../lib/utils';
import type { components } from '../../../api/v2/schema';

type WorkerHealthStatus = components['schemas']['WorkerHealthStatus'];

interface WorkerHealthProps {
  healthStatus: WorkerHealthStatus;
}

function WorkerHealth({ healthStatus }: WorkerHealthProps) {
  return (
    <div className="relative">
      <div
        className={cn(
          "w-2 h-2 rounded-full transition-colors duration-300",
          healthStatus === 'healthy' && "bg-success",
          healthStatus === 'warning' && "bg-warning",
          healthStatus === 'unhealthy' && "bg-error"
        )}
      />
      {healthStatus === 'healthy' && (
        <div className="absolute inset-0 rounded-full bg-success animate-ping opacity-75" />
      )}
    </div>
  );
}

export default WorkerHealth;