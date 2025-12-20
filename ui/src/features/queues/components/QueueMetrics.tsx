import React from 'react';
import { Layers, Play, Clock, BarChart3, Activity } from 'lucide-react';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '../../../components/ui/tooltip';

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
  // Define metric cards data
  const metricCards = [
    {
      title: 'Global Queues',
      value: metrics.globalQueues,
      icon: <Layers className="h-5 w-5 text-blue-500" />,
      tooltip: 'Number of global queues (shared across multiple DAGs with maxConcurrency limits)',
    },
    {
      title: 'DAG-based Queues',
      value: metrics.dagBasedQueues,
      icon: <Layers className="h-5 w-5 text-muted-foreground" />,
      tooltip: 'Number of DAG-based queues (each DAG has its own queue with maxActiveRuns limit, default 1)',
    },
    {
      title: 'Active Queues',
      value: metrics.activeQueues,
      icon: <Activity className="h-5 w-5 text-green-500" />,
      tooltip: 'Number of queues currently with running or queued DAG runs',
    },
    {
      title: 'Running',
      value: metrics.totalRunning,
      icon: <Play className="h-5 w-5 text-green-500" />,
      tooltip: 'Total number of DAG runs currently executing across all queues',
    },
    {
      title: 'Queued',
      value: metrics.totalQueued,
      icon: <Clock className="h-5 w-5 text-purple-500" />,
      tooltip: 'Total number of DAG runs waiting to be executed across all queues',
    },
    {
      title: 'Utilization',
      value: `${metrics.utilization}%`,
      icon: <BarChart3 className="h-5 w-5 text-orange-500" />,
      tooltip: 'Percentage of global queue capacity being used (total running DAG runs ÷ global queue maxConcurrency)',
    },
  ];

  return (
    <div className="border rounded bg-card flex-shrink-0">
      {/* Dense metrics */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 divide-x divide-y lg:divide-y-0">
        {metricCards.map((card) => (
          <Tooltip key={card.title}>
            <TooltipTrigger asChild>
              <div className="p-2 sm:p-3 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 sm:gap-2 cursor-help">
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
            </TooltipTrigger>
            <TooltipContent>
              <p className="max-w-xs">{card.tooltip}</p>
            </TooltipContent>
          </Tooltip>
        ))}
      </div>
    </div>
  );
}

export default QueueMetrics;
