/**
 * TimelineChart component visualizes the execution timeline of a DAG run.
 *
 * Features:
 * - Clean horizontal bar chart showing step execution
 * - Status-based color coding for each step
 * - Tooltips with step details on hover
 * - Works for both running and completed DAGs
 *
 * @module features/dags/components/visualization
 */
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import dayjs from '@/lib/dayjs';
import React, { useMemo } from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the TimelineChart component
 */
type Props = {
  /** DAG run details containing execution information */
  status: components['schemas']['DAGRunDetails'];
};

/** Format for displaying timestamps in tooltips */
const timeFormat = 'HH:mm:ss';

/**
 * Get status label for display
 */
function getStatusLabel(status: NodeStatus): string {
  switch (status) {
    case NodeStatus.NotStarted:
      return 'Not Started';
    case NodeStatus.Running:
      return 'Running';
    case NodeStatus.Success:
      return 'Success';
    case NodeStatus.Failed:
      return 'Failed';
    case NodeStatus.Aborted:
      return 'Aborted';
    case NodeStatus.Skipped:
      return 'Skipped';
    case NodeStatus.PartialSuccess:
      return 'Partial Success';
    default:
      return 'Unknown';
  }
}

/**
 * Calculate duration between two timestamps
 */
function calculateDuration(startMs: number, endMs: number): string {
  const durationMs = endMs - startMs;

  if (durationMs < 1000) {
    return `${durationMs}ms`;
  } else if (durationMs < 60000) {
    return `${(durationMs / 1000).toFixed(1)}s`;
  } else if (durationMs < 3600000) {
    const minutes = Math.floor(durationMs / 60000);
    const seconds = Math.floor((durationMs % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
  } else {
    const hours = Math.floor(durationMs / 3600000);
    const minutes = Math.floor((durationMs % 3600000) / 60000);
    return `${hours}h ${minutes}m`;
  }
}

/**
 * Status colors matching the Graph component (sepia theme)
 */
const statusColors: Record<NodeStatus, { bg: string; border: string }> = {
  [NodeStatus.NotStarted]: { bg: '#c8bfb0', border: '#c8bfb0' },
  [NodeStatus.Running]: { bg: '#7da87d', border: '#7da87d' },
  [NodeStatus.Failed]: { bg: '#c4726a', border: '#c4726a' },
  [NodeStatus.Aborted]: { bg: '#d4a574', border: '#d4a574' },
  [NodeStatus.Success]: { bg: '#7da87d', border: '#7da87d' },
  [NodeStatus.Skipped]: { bg: '#6b635a', border: '#6b635a' },
  [NodeStatus.PartialSuccess]: { bg: '#c4956a', border: '#c4956a' },
  [NodeStatus.Waiting]: { bg: '#f59e0b', border: '#f59e0b' },
};

/**
 * Get color for a node status
 */
function getStatusColor(status: NodeStatus): { bg: string; border: string } {
  return statusColors[status] || { bg: '#6b7280', border: '#6b7280' };
}

type TimelineItem = {
  name: string;
  startMs: number;
  endMs: number;
  status: NodeStatus;
  node: components['schemas']['Node'];
};

/**
 * TimelineChart component renders a horizontal bar chart showing step execution
 */
function TimelineChart({ status }: Props) {
  const { items, timelineStart, timelineEnd, timeMarkers } = useMemo(() => {
    const now = Date.now();
    const validItems: TimelineItem[] = [];

    (status.nodes || []).forEach((node) => {
      // Skip steps that haven't started
      if (!node.startedAt || node.startedAt === '-') {
        return;
      }

      const startMs = dayjs(node.startedAt).valueOf();
      let endMs: number;

      // Use current time for running steps
      if (!node.finishedAt || node.finishedAt === '-') {
        endMs = now;
      } else {
        endMs = dayjs(node.finishedAt).valueOf();
      }

      // Validate
      if (isNaN(startMs) || isNaN(endMs)) return;
      if (endMs < startMs) endMs = startMs + 100;

      validItems.push({
        name: node.step.name,
        startMs,
        endMs,
        status: node.status,
        node,
      });
    });

    // Sort by start time
    validItems.sort((a, b) => a.startMs - b.startMs);

    if (validItems.length === 0) {
      return { items: [], timelineStart: 0, timelineEnd: 0, timeMarkers: [] };
    }

    // Calculate timeline bounds
    const minStart = Math.min(...validItems.map((i) => i.startMs));
    const maxEnd = Math.max(...validItems.map((i) => i.endMs));
    const range = maxEnd - minStart;
    const padding = Math.max(range * 0.02, 500);

    const start = minStart - padding;
    const end = maxEnd + padding;

    // Generate time markers
    const totalRange = end - start;
    const markerCount = 5;
    const markers: { position: number; label: string }[] = [];

    for (let i = 0; i <= markerCount; i++) {
      const time = start + (totalRange * i) / markerCount;
      markers.push({
        position: (i / markerCount) * 100,
        label: dayjs(time).format(timeFormat),
      });
    }

    return {
      items: validItems,
      timelineStart: start,
      timelineEnd: end,
      timeMarkers: markers,
    };
  }, [status.nodes]);

  // Don't render if there are no items
  if (items.length === 0) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No step execution data available.
      </div>
    );
  }

  const totalRange = timelineEnd - timelineStart;

  return (
    <div className="w-full bg-card rounded-md border border-border overflow-hidden">
      {/* Time axis header */}
      <div className="relative h-6 bg-muted border-b border-border">
        {timeMarkers.map((marker, idx) => (
          <div
            key={idx}
            className="absolute top-0 h-full flex items-center"
            style={{ left: `${marker.position}%`, transform: 'translateX(-50%)' }}
          >
            <span className="text-[10px] text-muted-foreground font-mono">
              {marker.label}
            </span>
          </div>
        ))}
      </div>

      {/* Timeline rows */}
      <div className="relative">
        {/* Grid lines */}
        <div className="absolute inset-0 pointer-events-none">
          {timeMarkers.map((marker, idx) => (
            <div
              key={idx}
              className="absolute top-0 bottom-0 border-l border-border/30"
              style={{ left: `${marker.position}%` }}
            />
          ))}
        </div>

        {/* Step rows */}
        {items.map((item, idx) => {
          const leftPercent = ((item.startMs - timelineStart) / totalRange) * 100;
          const widthPercent = ((item.endMs - item.startMs) / totalRange) * 100;
          const colors = getStatusColor(item.status);
          const isRunning = item.status === NodeStatus.Running;

          return (
            <div
              key={item.name}
              className={`relative h-8 flex items-center ${
                idx % 2 === 0 ? 'bg-background' : 'bg-muted/20'
              }`}
            >
              {/* Step name label */}
              <div className="absolute left-2 z-10 text-xs font-medium text-foreground truncate max-w-[120px]">
                {item.name}
              </div>

              {/* Timeline bar */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <div
                    className={`absolute h-5 rounded cursor-pointer transition-opacity hover:opacity-80 ${
                      isRunning ? 'animate-pulse' : ''
                    }`}
                    style={{
                      left: `calc(${leftPercent}% + 130px)`,
                      width: `calc(${Math.max(widthPercent, 0.5)}% - 130px)`,
                      minWidth: '4px',
                      backgroundColor: colors.bg,
                      borderLeft: `2px solid ${colors.border}`,
                    }}
                  />
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-xs">
                  <div className="space-y-1">
                    <div className="font-semibold">{item.name}</div>
                    {item.node.step.description && (
                      <div className="text-xs text-muted-foreground">
                        {item.node.step.description}
                      </div>
                    )}
                    <div className="text-xs">
                      Status: <span className="font-medium">{getStatusLabel(item.status)}</span>
                    </div>
                    <div className="text-xs">
                      Duration: <span className="font-mono">{calculateDuration(item.startMs, item.endMs)}</span>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {dayjs(item.startMs).format('HH:mm:ss')} → {dayjs(item.endMs).format('HH:mm:ss')}
                    </div>
                    {item.node.error && (
                      <div className="text-xs text-destructive">
                        Error: {item.node.error}
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default TimelineChart;
