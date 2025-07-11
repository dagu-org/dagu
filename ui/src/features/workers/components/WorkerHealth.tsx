import React from 'react';
import { cn } from '../../../lib/utils';

interface WorkerHealthProps {
  lastHeartbeat: string;
}

function WorkerHealth({ lastHeartbeat }: WorkerHealthProps) {
  const [health, setHealth] = React.useState<'healthy' | 'warning' | 'offline'>('offline');

  React.useEffect(() => {
    const updateHealth = () => {
      if (!lastHeartbeat) {
        setHealth('offline');
        return;
      }

      const time = new Date(lastHeartbeat).getTime();
      const now = new Date().getTime();
      const secondsAgo = Math.floor((now - time) / 1000);

      if (secondsAgo < 10) {
        setHealth('healthy');
      } else if (secondsAgo < 60) {
        setHealth('warning');
      } else {
        setHealth('offline');
      }
    };

    updateHealth();
    const interval = setInterval(updateHealth, 1000);
    return () => clearInterval(interval);
  }, [lastHeartbeat]);

  return (
    <div className="relative">
      <div
        className={cn(
          "w-2 h-2 rounded-full transition-colors duration-300",
          health === 'healthy' && "bg-green-500",
          health === 'warning' && "bg-yellow-500",
          health === 'offline' && "bg-red-500"
        )}
      />
      {health === 'healthy' && (
        <div className="absolute inset-0 rounded-full bg-green-500 animate-ping opacity-75" />
      )}
    </div>
  );
}

export default WorkerHealth;