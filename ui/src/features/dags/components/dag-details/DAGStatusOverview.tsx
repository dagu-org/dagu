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
      {/* Parameters - Show at the top if present */}
      {status.params && (
        <div className="border-b border-slate-200 dark:border-slate-700 pb-3">
          <div className="flex items-center mb-1.5">
            <Terminal className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <span className="text-xs font-semibold text-slate-700 dark:text-slate-300">
              Parameters
            </span>
          </div>
          <div className="p-2 bg-slate-200 dark:bg-slate-700 rounded-md font-medium text-xs text-slate-800 dark:text-slate-200 font-mono max-h-[100px] overflow-y-auto w-full border">
            {status.params}
          </div>
        </div>
      )}

      {/* Status Section - Desktop */}
      <div className="hidden md:flex items-center justify-between border-b border-slate-200 dark:border-slate-700 pb-2">
        <div className="flex items-center gap-2">
          <StatusChip status={status.status} size="md">
            {status.statusLabel}
          </StatusChip>

        </div>

        {status.dagRunId && (
          <div className="flex items-center gap-1.5">
            <div className="flex items-center">
              <Hash className="h-3 w-3 mr-0.5 text-slate-500 dark:text-slate-400" />
              <span className="text-xs font-mono bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded text-slate-700 dark:text-slate-300">
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
              className="inline-flex items-center text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 transition-colors duration-200 cursor-pointer"
              title="Click to view log (Cmd/Ctrl+Click to open in new tab)"
            >
              <FileText className="h-3.5 w-3.5" />
            </a>
          </div>
        )}
      </div>

      {/* Status Section - Mobile */}
      <div className="md:hidden border-b border-slate-200 dark:border-slate-700 pb-2 space-y-2">
        <div>
          <StatusChip status={status.status} size="md">
            {status.statusLabel}
          </StatusChip>
        </div>


        {status.dagRunId && (
          <div className="space-y-1">
            <div className="flex items-center">
              <Hash className="h-3 w-3 mr-0.5 text-slate-500 dark:text-slate-400" />
              <span className="text-xs font-mono bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded text-slate-700 dark:text-slate-300">
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
                className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors duration-200 cursor-pointer"
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
      <div className="border-b border-slate-200 dark:border-slate-700 pb-2">
        <div className="flex flex-col md:flex-row flex-wrap items-start gap-1">
          {status.queuedAt && (
            <div className="flex items-center">
              <Clock className="w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
              <LabeledItem label="Queued">
                <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                  {formatTimestamp(status.queuedAt)}
                </span>
              </LabeledItem>
            </div>
          )}

          <div className="flex items-center">
            <Calendar className="w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Started">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                {formatTimestamp(status.startedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Clock className="w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Finished">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                {formatTimestamp(status.finishedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Timer className="w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Duration">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs flex items-center gap-1">
                {currentDuration}
                {isRunning && (
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-lime-500 animate-pulse" />
                )}
              </span>
            </LabeledItem>
          </div>
        </div>
      </div>

      {/* Node Status Summary */}
      <div className="border-b border-slate-200 dark:border-slate-700 pb-2">
        <div className="flex items-center mb-1">
          <Layers className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
          <span className="text-xs font-semibold text-slate-700 dark:text-slate-300">
            Node Status
          </span>
        </div>

        <div className="flex flex-wrap gap-2">
          <div className="flex items-center">
            <Info className="h-3 w-3 mr-1 text-slate-500 dark:text-slate-400" />
            <span className="text-xs text-slate-600 dark:text-slate-400">
              Total: {totalNodes}
            </span>
          </div>

          {nodeStatus.finished && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-green-500"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Success: {nodeStatus.finished}
              </span>
            </div>
          )}

          {nodeStatus.running && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-lime-500 animate-pulse"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Running: {nodeStatus.running}
              </span>
            </div>
          )}

          {nodeStatus.failed && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-red-500"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Failed: {nodeStatus.failed}
              </span>
            </div>
          )}

          {nodeStatus.queued && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-purple-500"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Queued: {nodeStatus.queued}
              </span>
            </div>
          )}

          {nodeStatus.not_started && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Not Started: {nodeStatus.not_started}
              </span>
            </div>
          )}

          {nodeStatus.skipped && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-slate-400"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Skipped: {nodeStatus.skipped}
              </span>
            </div>
          )}

          {nodeStatus.canceled && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-pink-400"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Canceled: {nodeStatus.canceled}
              </span>
            </div>
          )}
        </div>

        {/* Progress bar */}
        {totalNodes && totalNodes > 0 && (
          <div className="mt-1.5 h-1.5 w-full bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
            {nodeStatus?.finished && (
              <div
                className="h-full bg-green-500 float-left"
                style={{
                  width: `${(nodeStatus.finished / totalNodes) * 100}%`,
                }}
              ></div>
            )}
            {nodeStatus.running && (
              <div
                className="h-full bg-lime-500 float-left animate-pulse"
                style={{ width: `${(nodeStatus.running / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.queued && (
              <div
                className="h-full bg-purple-500 float-left"
                style={{ width: `${(nodeStatus.queued / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.failed && (
              <div
                className="h-full bg-red-500 float-left"
                style={{ width: `${(nodeStatus.failed / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.skipped && (
              <div
                className="h-full bg-slate-400 float-left"
                style={{ width: `${(nodeStatus.skipped / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStatus.canceled && (
              <div
                className="h-full bg-pink-400 float-left"
                style={{
                  width: `${(nodeStatus.canceled / totalNodes) * 100}%`,
                }}
              ></div>
            )}
          </div>
        )}

        {/* Execution controls indicator */}
        {isRunning && (
          <div className="mt-1.5 flex items-center text-xs text-slate-600 dark:text-slate-400">
            <PlayCircle className="h-3 w-3 mr-1 text-lime-500" />
            <span>Execution in progress</span>
          </div>
        )}
        {status.status === Status.Queued && (
          <div className="mt-1.5 flex items-center text-xs text-slate-600 dark:text-slate-400">
            <Clock className="h-3 w-3 mr-1 text-purple-500" />
            <span>DAGRun is queued for execution</span>
          </div>
        )}
        {status.status === Status.Cancelled && (
          <div className="mt-1.5 flex items-center text-xs text-slate-600 dark:text-slate-400">
            <StopCircle className="h-3 w-3 mr-1 text-pink-400" />
            <span>Execution was cancelled</span>
          </div>
        )}
      </div>

      {/* DAGRun-level Precondition Errors */}
      {status.preconditions?.some(
        (cond: components['schemas']['Condition']) => cond.error
      ) && (
        <div className="border-b border-slate-200 dark:border-slate-700 pb-2">
          <div className="flex items-center mb-1">
            <Info className="h-3.5 w-3.5 mr-1 text-amber-500 dark:text-amber-400" />
            <span className="text-xs font-semibold text-amber-600 dark:text-amber-400">
              DAGRun Precondition Unmet
            </span>
          </div>
          <div className="space-y-2">
            {status.preconditions
              ?.filter((cond: components['schemas']['Condition']) => cond.error)
              .map((cond: components['schemas']['Condition'], idx: number) => (
                <div
                  key={idx}
                  className="p-1.5 bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-800 rounded-md text-xs text-amber-600 dark:text-amber-400 font-medium"
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
