/**
 * DAGStatusOverview component displays summary information about a DAG run.
 *
 * @module features/dags/components/dag-details
 */
import dayjs from '@/lib/dayjs';
import {
  Calendar,
  Clock,
  FileText,
  Hash,
  Info,
  Layers,
  PlayCircle,
  StopCircle,
  Terminal,
  Timer,
} from 'lucide-react';
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import LabeledItem from '../../../../ui/LabeledItem';
import StatusChip from '../../../../ui/StatusChip';

/**
 * Props for the DAGStatusOverview component
 */
type Props = {
  /** DAG dagRun details */
  status?: components['schemas']['DAGRunDetails'];
  /** DAG file name */
  fileName: string;
  /** DAGRun ID of the execution */
  dagRunId?: string;
  /** Function to open log viewer */
  onViewLog?: (dagRunId: string) => void;
};

/**
 * DAGStatusOverview displays summary information about a DAG run
 * including status, request ID, timestamps, and parameters
 */
function DAGStatusOverview({
  status,
  fileName,
  dagRunId = '',
  onViewLog,
}: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  if (dagRunId) {
    searchParams.set('dagRunId', dagRunId);
  }
  const url = `/dags/${fileName}/dagRun-log?${searchParams.toString()}`;

  // State to store current duration for live updates
  const [currentDuration, setCurrentDuration] = React.useState<string>('-');

  // Don't render if no status is provided
  if (!status) {
    return null;
  }

  // Format timestamps for better readability if they exist
  const formatTimestamp = (timestamp: string | undefined) => {
    if (!timestamp || timestamp === '-') return '-';
    try {
      return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss Z');
    } catch {
      return timestamp;
    }
  };

  // Calculate duration between start and end times
  const calculateDuration = () => {
    if (!status.startedAt || status.startedAt === '-') return '-';

    const end =
      status.finishedAt && status.finishedAt !== '-'
        ? dayjs(status.finishedAt)
        : dayjs();

    const start = dayjs(status.startedAt);
    const diff = end.diff(start, 'second');

    const hours = Math.floor(diff / 3600);
    const minutes = Math.floor((diff % 3600) / 60);
    const seconds = diff % 60;

    return `${hours > 0 ? `${hours}h ` : ''}${minutes > 0 ? `${minutes}m ` : ''}${seconds}s`;
  };

  // Determine if the DAG is currently running
  const isRunning = status.status === Status.Running;

  // Auto-update duration every second for running DAGs
  React.useEffect(() => {
    if (isRunning && status.startedAt) {
      // Initial calculation
      setCurrentDuration(calculateDuration());

      // Set up interval to update duration every second
      const intervalId = setInterval(() => {
        setCurrentDuration(calculateDuration());
      }, 1000);

      // Clean up interval on unmount or when status changes
      return () => clearInterval(intervalId);
    } else {
      // For non-running DAGs, calculate once
      setCurrentDuration(calculateDuration());
    }
  }, [isRunning, status.startedAt, status.finishedAt]);

  // Count nodes by status
  const nodeStatus = status.nodes?.reduce(
    (acc, node) => {
      const statusKey = node.statusLabel.toLowerCase().replace(' ', '_');
      acc[statusKey] = (acc[statusKey] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>
  );

  // Calculate total nodes
  const totalNodes = status.nodes?.length;

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
        <div className="p-2 bg-accent rounded-md text-xs font-mono h-[40px] overflow-y-auto w-full border">
          {status.params ? (
            <span className="font-medium text-foreground">{status.params}</span>
          ) : (
            <span className="text-muted-foreground italic">No parameters</span>
          )}
        </div>
      </div>

      {/* Status Section - Desktop */}
      <div className="hidden md:flex items-center justify-between pb-2">
        <div className="flex items-center gap-2">
          <StatusChip status={status.status} size="md">
            {status.statusLabel}
          </StatusChip>
        </div>

        {status.dagRunId && (
          <div className="flex items-center gap-1.5">
            <div className="flex items-center">
              <Hash className="h-3 w-3 mr-0.5 text-muted-foreground" />
              <span className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded text-foreground/90">
                {status.dagRunId}
              </span>
            </div>

            <a
              href={url}
              onClick={(e) => {
                if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                  e.preventDefault();
                  onViewLog(status.dagRunId);
                }
              }}
              className="inline-flex items-center text-muted-foreground hover:text-foreground transition-colors duration-200 cursor-pointer"
              title="Click to view log (Cmd/Ctrl+Click to open in new tab)"
            >
              <FileText className="h-3.5 w-3.5" />
            </a>
          </div>
        )}
      </div>

      {/* Status Section - Mobile */}
      <div className="md:hidden pb-2 space-y-2">
        <div>
          <StatusChip status={status.status} size="md">
            {status.statusLabel}
          </StatusChip>
        </div>

        {status.dagRunId && (
          <div className="space-y-1">
            <div className="flex items-center">
              <Hash className="h-3 w-3 mr-0.5 text-muted-foreground" />
              <span className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded text-foreground/90">
                {status.dagRunId}
              </span>
            </div>

            <div>
              <a
                href={url}
                onClick={(e) => {
                  if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                    e.preventDefault();
                    onViewLog(status.dagRunId);
                  }
                }}
                className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors duration-200 cursor-pointer"
                title="Click to view log (Cmd/Ctrl+Click to open in new tab)"
              >
                <FileText className="h-3.5 w-3.5" />
                <span>View Log</span>
              </a>
            </div>
          </div>
        )}
      </div>

      {/* Timing Information */}
      <div className="pb-2">
        <div className="flex flex-col md:flex-row flex-wrap items-start gap-1">
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

          {nodeStatus.finished && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-success"></div>
              <span className="text-xs text-muted-foreground">
                Success: {nodeStatus.finished}
              </span>
            </div>
          )}

          {nodeStatus.running && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-success animate-pulse"></div>
              <span className="text-xs text-muted-foreground">
                Running: {nodeStatus.running}
              </span>
            </div>
          )}

          {nodeStatus.failed && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-error"></div>
              <span className="text-xs text-muted-foreground">
                Failed: {nodeStatus.failed}
              </span>
            </div>
          )}

          {nodeStatus.queued && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-info"></div>
              <span className="text-xs text-muted-foreground">
                Queued: {nodeStatus.queued}
              </span>
            </div>
          )}

          {nodeStatus.not_started && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-accent"></div>
              <span className="text-xs text-muted-foreground">
                Not Started: {nodeStatus.not_started}
              </span>
            </div>
          )}

          {nodeStatus.skipped && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-muted-foreground"></div>
              <span className="text-xs text-muted-foreground">
                Skipped: {nodeStatus.skipped}
              </span>
            </div>
          )}

          {nodeStatus.aborted && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-pink-400"></div>
              <span className="text-xs text-muted-foreground">
                Aborted: {nodeStatus.aborted}
              </span>
            </div>
          )}
        </div>

        {/* Progress bar */}
        {totalNodes && totalNodes > 0 && (
          <div className="mt-1.5 h-1.5 w-full bg-accent rounded-full overflow-hidden">
            {nodeStatus?.finished && (
              <div
                className="h-full bg-success float-left"
                style={{
                  width: `${(nodeStatus.finished / totalNodes) * 100}%`,
                }}
              ></div>
            )}
            {nodeStatus.running && (
              <div
                className="h-full bg-success float-left animate-pulse"
                style={{ width: `${(nodeStatus.running / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.queued && (
              <div
                className="h-full bg-info float-left"
                style={{ width: `${(nodeStatus.queued / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.failed && (
              <div
                className="h-full bg-error float-left"
                style={{ width: `${(nodeStatus.failed / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.skipped && (
              <div
                className="h-full bg-muted-foreground float-left"
                style={{ width: `${(nodeStatus.skipped / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.aborted && (
              <div
                className="h-full bg-pink-400 float-left"
                style={{
                  width: `${(nodeStatus.aborted / totalNodes) * 100}%`,
                }}
              ></div>
            )}
          </div>
        )}

        {/* Execution controls indicator */}
        {isRunning && (
          <div className="mt-1.5 flex items-center text-xs text-muted-foreground">
            <PlayCircle className="h-3 w-3 mr-1 text-success" />
            <span>Execution in progress</span>
          </div>
        )}
        {status.status === Status.Queued && (
          <div className="mt-1.5 flex items-center text-xs text-muted-foreground">
            <Clock className="h-3 w-3 mr-1 text-info" />
            <span>DAGRun is queued for execution</span>
          </div>
        )}
        {status.status === Status.Aborted && (
          <div className="mt-1.5 flex items-center text-xs text-muted-foreground">
            <StopCircle className="h-3 w-3 mr-1 text-pink-400" />
            <span>Execution was aborted</span>
          </div>
        )}
      </div>

      {/* DAGRun-level Precondition Errors */}
      {status.preconditions?.some(
        (cond: components['schemas']['Condition']) => cond.error
      ) && (
        <div className="pb-2">
          <div className="flex items-center mb-1">
            <Info className="h-3.5 w-3.5 mr-1 text-warning" />
            <span className="text-xs font-semibold text-warning">
              DAGRun Precondition Unmet
            </span>
          </div>
          <div className="space-y-2">
            {status.preconditions
              ?.filter((cond: components['schemas']['Condition']) => cond.error)
              .map((cond: components['schemas']['Condition'], idx: number) => (
                <div
                  key={idx}
                  className="p-1.5 bg-warning-muted border border-warning/20 rounded-md text-xs text-warning font-medium"
                >
                  <div className="mb-0.5">Condition: {cond.condition}</div>
                  <div className="mb-0.5">Expected: {cond.expected}</div>
                  <div>Error: {cond.error}</div>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}

export default DAGStatusOverview;
