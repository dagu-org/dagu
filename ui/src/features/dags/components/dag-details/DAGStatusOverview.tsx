/**
 * DAGStatusOverview component displays summary information about a DAG run.
 *
 * @module features/dags/components/dag-details
 */
import dayjs from '@/lib/dayjs';
import {
  Calendar,
  Clock,
  Hash,
  Info,
  Layers,
  LucideIcon,
  PlayCircle,
  Server,
  StopCircle,
  Terminal,
  Timer,
  Zap,
} from 'lucide-react';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { components, Status } from '../../../../api/v2/schema';
import LabeledItem from '../../../../ui/LabeledItem';
import { TriggerTypeIndicator } from '../common/TriggerTypeIndicator';

type Props = {
  status?: components['schemas']['DAGRunDetails'];
  onViewLog?: (dagRunId: string) => void;
};

type NodeStatusConfig = {
  key: string;
  label: string;
  colorClass: string;
  animate?: boolean;
};

const NODE_STATUS_CONFIG: NodeStatusConfig[] = [
  { key: 'succeeded', label: 'Success', colorClass: 'bg-success' },
  { key: 'running', label: 'Running', colorClass: 'bg-success', animate: true },
  { key: 'failed', label: 'Failed', colorClass: 'bg-error' },
  { key: 'queued', label: 'Queued', colorClass: 'bg-info' },
  { key: 'not_started', label: 'Not Started', colorClass: 'bg-accent' },
  { key: 'skipped', label: 'Skipped', colorClass: 'bg-muted-foreground' },
  { key: 'aborted', label: 'Aborted', colorClass: 'bg-pink-400' },
  { key: 'waiting', label: 'Waiting', colorClass: 'bg-amber-500', animate: true },
  { key: 'rejected', label: 'Rejected', colorClass: 'bg-red-600' },
];

type ExecutionStatusConfig = {
  status: Status;
  icon: LucideIcon;
  iconClass: string;
  message: string;
};

const EXECUTION_STATUS_CONFIG: ExecutionStatusConfig[] = [
  { status: Status.Running, icon: PlayCircle, iconClass: 'text-success', message: 'Execution in progress' },
  { status: Status.Queued, icon: Clock, iconClass: 'text-info', message: 'DAGRun is queued for execution' },
  { status: Status.Aborted, icon: StopCircle, iconClass: 'text-pink-400', message: 'Execution was aborted' },
  { status: Status.Waiting, icon: Clock, iconClass: 'text-amber-500', message: 'Waiting for approval' },
  { status: Status.Rejected, icon: StopCircle, iconClass: 'text-red-600', message: 'Execution was rejected' },
];

function formatTimestamp(timestamp: string | undefined): string {
  if (!timestamp || timestamp === '-') {
    return '-';
  }
  const parsed = dayjs(timestamp);
  if (!parsed.isValid()) {
    return timestamp;
  }
  return parsed.format('YYYY-MM-DD HH:mm:ss Z');
}

type PreconditionErrorsProps = {
  preconditions?: components['schemas']['Condition'][];
};

function PreconditionErrors({ preconditions }: PreconditionErrorsProps): React.JSX.Element | null {
  const errors = preconditions?.filter((cond) => cond.error);

  if (!errors || errors.length === 0) {
    return null;
  }

  return (
    <div className="pb-2">
      <div className="flex items-center mb-1">
        <Info className="h-3.5 w-3.5 mr-1 text-warning" />
        <span className="text-xs font-semibold text-warning">
          DAGRun Precondition Unmet
        </span>
      </div>
      <div className="space-y-2">
        {errors.map((cond, idx) => (
          <div
            key={idx}
            className="p-1.5 bg-warning-muted border border-warning/20 rounded-md text-xs text-warning font-medium whitespace-normal break-words"
          >
            <div className="mb-0.5 break-words">Condition: {cond.condition}</div>
            <div className="mb-0.5 break-words">Expected: {cond.expected}</div>
            <div className="break-words">Error: {cond.error}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function formatDuration(startedAt: string | undefined, finishedAt: string | undefined): string {
  if (!startedAt || startedAt === '-') {
    return '-';
  }

  const start = dayjs(startedAt);
  if (!start.isValid()) {
    return '-';
  }

  const end = finishedAt && finishedAt !== '-' ? dayjs(finishedAt) : dayjs();
  if (!end.isValid()) {
    return '-';
  }

  const diff = end.diff(start, 'second');
  if (diff < 0) {
    return '-';
  }

  const hours = Math.floor(diff / 3600);
  const minutes = Math.floor((diff % 3600) / 60);
  const seconds = diff % 60;

  const parts: string[] = [];
  if (hours > 0) {
    parts.push(`${hours}h`);
  }
  if (minutes > 0) {
    parts.push(`${minutes}m`);
  }
  parts.push(`${seconds}s`);

  return parts.join(' ');
}

function DAGStatusOverview({ status, onViewLog }: Props): React.JSX.Element | null {
  const [currentDuration, setCurrentDuration] = useState<string>('-');

  const isRunning = status?.status === Status.Running;

  const calculateDuration = useCallback((): string => {
    return formatDuration(status?.startedAt, status?.finishedAt);
  }, [status?.startedAt, status?.finishedAt]);

  useEffect(() => {
    setCurrentDuration(calculateDuration());

    if (isRunning && status?.startedAt) {
      const intervalId = setInterval(() => {
        setCurrentDuration(calculateDuration());
      }, 1000);
      return () => clearInterval(intervalId);
    }
  }, [isRunning, status?.startedAt, calculateDuration]);

  const nodeStatus = useMemo(() => {
    return status?.nodes?.reduce(
      (acc, node) => {
        const statusKey = node.statusLabel.toLowerCase().replace(' ', '_');
        acc[statusKey] = (acc[statusKey] || 0) + 1;
        return acc;
      },
      {} as Record<string, number>
    );
  }, [status?.nodes]);

  const totalNodes = status?.nodes?.length;

  if (!status) {
    return null;
  }

  return (
    <div className="space-y-3">
      {/* Parameters - Always show to prevent layout jumping */}
      <div className="pb-3">
        <div className="flex items-center mb-1.5">
          <Terminal className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
          <span className="text-xs font-semibold text-foreground/90">
            Parameters
          </span>
        </div>
        <div className="flex items-center p-2 bg-accent rounded-md text-xs font-mono h-[40px] overflow-y-auto w-full border">
          {status.params ? (
            <span className="font-medium text-foreground">{status.params}</span>
          ) : (
            <span className="text-muted-foreground italic">No parameters</span>
          )}
        </div>
      </div>

      {/* Timing Information */}
      <div className="pb-2 space-y-2">
        {/* Row 1: Time info */}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {status.queuedAt && (
            <div className="flex items-center">
              <Clock className="w-3.5 mr-1 text-muted-foreground" />
              <LabeledItem label="Queued">
                <span className="font-medium text-foreground/90 text-xs">
                  {formatTimestamp(status.queuedAt)}
                </span>
              </LabeledItem>
            </div>
          )}

          <div className="flex items-center">
            <Calendar className="w-3.5 mr-1 text-muted-foreground" />
            <LabeledItem label="Started">
              <span className="font-medium text-foreground/90 text-xs">
                {formatTimestamp(status.startedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Clock className="w-3.5 mr-1 text-muted-foreground" />
            <LabeledItem label="Finished">
              <span className="font-medium text-foreground/90 text-xs">
                {formatTimestamp(status.finishedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Timer className="w-3.5 mr-1 text-muted-foreground" />
            <LabeledItem label="Duration">
              <span className="font-medium text-foreground/90 text-xs flex items-center gap-1">
                {currentDuration}
                {isRunning && (
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
                )}
              </span>
            </LabeledItem>
          </div>
        </div>

        {/* Row 2: Trigger, Worker, and Run ID */}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {status.triggerType && (
            <div className="flex items-center">
              <Zap className="w-3.5 mr-1 text-muted-foreground" />
              <LabeledItem label="Trigger">
                <TriggerTypeIndicator type={status.triggerType} size={12} />
              </LabeledItem>
            </div>
          )}

          {status.workerId && (
            <div className="flex items-center">
              <Server className="w-3.5 mr-1 text-muted-foreground" />
              <LabeledItem label="Worker">
                <span
                  className="font-medium text-foreground/90 text-xs truncate inline-block max-w-[250px]"
                  title={status.workerId}
                >
                  {status.workerId}
                </span>
              </LabeledItem>
            </div>
          )}

          {status.dagRunId && (
            <div className="flex items-center">
              <Hash className="w-3.5 mr-1 text-muted-foreground" />
              <LabeledItem label="Run ID">
                <span className="font-medium text-foreground/90 text-xs">
                  {status.dagRunId}
                </span>
              </LabeledItem>
            </div>
          )}

          {status.dagRunId && onViewLog && (
            <button
              onClick={() => onViewLog(status.dagRunId)}
              className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md border border-border shadow-sm bg-card hover:bg-muted transition-colors duration-200 cursor-pointer"
              title="View Scheduler Log"
            >
              <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
              <span>Scheduler Log</span>
            </button>
          )}
        </div>
      </div>

      {/* Node Status Summary */}
      <div className="pb-2">
        <div className="flex items-center mb-1">
          <Layers className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
          <span className="text-xs font-semibold text-foreground/90">
            Node Status
          </span>
        </div>

        <div className="flex flex-wrap gap-2">
          <div className="flex items-center">
            <Info className="h-3 w-3 mr-1 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">
              Total: {totalNodes}
            </span>
          </div>

          {NODE_STATUS_CONFIG.map(({ key, label, colorClass, animate }) => {
            const count = nodeStatus?.[key];
            if (!count) {
              return null;
            }
            return (
              <div key={key} className="flex items-center">
                <div
                  className={`h-2 w-2 mr-1 rounded-full ${colorClass} ${animate ? 'animate-pulse' : ''}`}
                />
                <span className="text-xs text-muted-foreground">
                  {label}: {count}
                </span>
              </div>
            );
          })}
        </div>

        {totalNodes && totalNodes > 0 && nodeStatus && (
          <div className="mt-1.5 h-1.5 w-full bg-accent rounded-full overflow-hidden">
            {NODE_STATUS_CONFIG.map(({ key, colorClass, animate }) => {
              const count = nodeStatus[key];
              if (!count) {
                return null;
              }
              return (
                <div
                  key={key}
                  className={`h-full ${colorClass} float-left ${animate ? 'animate-pulse' : ''}`}
                  style={{ width: `${(count / totalNodes) * 100}%` }}
                />
              );
            })}
          </div>
        )}

        {EXECUTION_STATUS_CONFIG.map(({ status: execStatus, icon: Icon, iconClass, message }) => {
          if (status.status !== execStatus) {
            return null;
          }
          return (
            <div key={execStatus} className="mt-1.5 flex items-center text-xs text-muted-foreground">
              <Icon className={`h-3 w-3 mr-1 ${iconClass}`} />
              <span>{message}</span>
            </div>
          );
        })}
      </div>

      <PreconditionErrors preconditions={status.preconditions} />
    </div>
  );
}

export default DAGStatusOverview;
