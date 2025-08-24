import React from 'react';
import { Layers, Play, Clock, BarChart3, Settings, GitBranch } from 'lucide-react';

interface QueueMetricsProps {
  metrics: {
    totalQueues: number;
    globalQueues: number;
    dagBasedQueues: number;
    totalRunning: number;
    totalQueued: number;
    totalActive: number;
    utilization: number;
  };
  isLoading?: boolean;
}

function QueueMetrics({ metrics, isLoading }: QueueMetricsProps) {
  // Define metric cards data
  const metricCards = [
    {
      title: 'Total Queues',
      value: metrics.totalQueues,
      icon: <Layers className="h-5 w-5 text-muted-foreground" />,
    },
    {
      title: 'Global',
      value: metrics.globalQueues,
      icon: <Settings className="h-5 w-5 text-blue-500" />,
    },
    {
      title: 'DAG-based',
      value: metrics.dagBasedQueues,
      icon: <GitBranch className="h-5 w-5 text-gray-500" />,
    },
    {
      title: 'Running',
      value: metrics.totalRunning,
      icon: <Play className="h-5 w-5 text-green-500" />,
    },
    {
      title: 'Queued',
      value: metrics.totalQueued,
      icon: <Clock className="h-5 w-5 text-purple-500" />,
    },
    {
      title: 'Utilization',
      value: `${metrics.utilization}%`,
      icon: <BarChart3 className="h-5 w-5 text-orange-500" />,
    },
  ];

  return (
    <div className="border rounded bg-card flex-shrink-0">
      {/* Dense metrics */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 divide-x divide-y lg:divide-y-0">
        {metricCards.map((card) => (
          <div
            key={card.title}
            className="p-2 sm:p-3 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 sm:gap-2"
          >
            <div className="flex items-center gap-1 sm:gap-2">
              {React.cloneElement(card.icon, {
                className: card.icon.props.className.replace(
                  'h-5 w-5',
                  'h-3 w-3'
                ),
              })}
              <span className="text-xs font-medium text-muted-foreground">
                {card.title}
              </span>
            </div>
            <span className="text-lg font-bold">
              {isLoading ? '-' : card.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default QueueMetrics;