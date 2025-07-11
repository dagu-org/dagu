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
          healthStatus === 'healthy' && "bg-green-500",
          healthStatus === 'warning' && "bg-yellow-500",
          healthStatus === 'unhealthy' && "bg-red-500"
        )}
      />
      {healthStatus === 'healthy' && (
        <div className="absolute inset-0 rounded-full bg-green-500 animate-ping opacity-75" />
      )}
    </div>
  );
}

export default WorkerHealth;