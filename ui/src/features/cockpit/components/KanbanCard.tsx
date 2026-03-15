import React, { useMemo } from 'react';
import { motion } from 'framer-motion';
import { components, Status } from '@/api/v1/schema';
import AutoRetryBadge from '@/features/dag-runs/components/common/AutoRetryBadge';
import dayjs from '@/lib/dayjs';
import StatusChip from '@/ui/StatusChip';
import Ticker from '@/ui/Ticker';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  run: DAGRunSummary;
  onClick: () => void;
}

function formatElapsed(run: DAGRunSummary): string {
  const start = run.startedAt ? new Date(run.startedAt).getTime() : 0;
  if (!start) return '';
  const end =
    run.status === Status.Running
      ? Date.now()
      : run.finishedAt
        ? new Date(run.finishedAt).getTime()
        : Date.now();
  const seconds = Math.floor((end - start) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (minutes < 60) return `${minutes}m ${secs}s`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}

function truncateParams(params: string | undefined, maxLen = 60): string {
  if (!params) return '';
  const clean =
    params.length > maxLen ? params.slice(0, maxLen) + '...' : params;
  return clean;
}

function formatScheduleTime(scheduleTime: string | undefined): string {
  if (!scheduleTime) return '';
  const parsed = dayjs(scheduleTime);
  if (!parsed.isValid()) return '';
  return parsed.format('MMM D, HH:mm:ss');
}

export function KanbanCard({ run, onClick }: Props): React.ReactElement {
  const params = useMemo(() => truncateParams(run.params), [run.params]);
  const scheduleTime = useMemo(
    () => formatScheduleTime(run.scheduleTime),
    [run.scheduleTime]
  );

  return (
    <motion.div
      layoutId={run.dagRunId}
      layout
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.95 }}
      transition={{ duration: 0.2, ease: 'easeOut' }}
    >
      <button
        onClick={onClick}
        className="w-full text-left p-2 rounded-md border border-border bg-card hover:bg-accent/50 transition-colors cursor-pointer"
      >
        <div className="mb-1 flex items-start justify-between gap-2">
          <span className="min-w-0 flex-1 truncate text-xs font-medium leading-tight">
            {run.name}
          </span>
          <div className="flex shrink-0 flex-col items-end gap-1">
            <StatusChip status={run.status} size="xs">
              {run.statusLabel}
            </StatusChip>
            <AutoRetryBadge
              count={run.autoRetryCount}
              limit={run.autoRetryLimit}
              className="text-[11px]"
            />
          </div>
        </div>
        {run.startedAt &&
          (run.status === Status.Running ? (
            <Ticker intervalMs={1000}>
              {() => (
                <div className="text-[11px] text-muted-foreground">
                  {formatElapsed(run)}
                </div>
              )}
            </Ticker>
          ) : (
            <div className="text-[11px] text-muted-foreground">
              {formatElapsed(run)}
            </div>
          ))}
        {scheduleTime && (
          <div className="text-[11px] text-muted-foreground">
            Scheduled {scheduleTime}
          </div>
        )}
        {params && (
          <div className="text-[11px] text-muted-foreground mt-0.5 truncate font-mono">
            {params}
          </div>
        )}
      </button>
    </motion.div>
  );
}
