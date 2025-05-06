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
import { components, Status } from '../../../../api/v2/schema';
import LabeledItem from '../../../../ui/LabeledItem';
import StatusChip from '../../../../ui/StatusChip';

/**
 * Props for the DAGStatusOverview component
 */
type Props = {
  /** DAG run details */
  status?: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileName: string;
  /** Request ID of the execution */
  requestId?: string;
  /** Function to open log viewer */
  onViewLog?: (requestId: string) => void;
};

/**
 * DAGStatusOverview displays summary information about a DAG run
 * including status, request ID, timestamps, and parameters
 */
function DAGStatusOverview({
  status,
  fileName,
  requestId = '',
  onViewLog,
}: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  if (requestId) {
    searchParams.set('requestId', requestId);
  }
  const url = `/dags/${fileName}/scheduler-log?${searchParams.toString()}`;

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

  // Count nodes by status
  const nodeStats = status.nodes?.reduce(
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
      {/* Status Section */}
      <div className="flex items-center justify-between border-b border-slate-200 dark:border-slate-700 pb-2">
        <div className="flex items-center gap-2">
          <StatusChip status={status.status} size="md">
            {status.statusLabel}
          </StatusChip>

          {status.pid && (
            <div className="flex items-center text-xs text-slate-500 dark:text-slate-400">
              <Terminal className="h-3 w-3 mr-0.5" />
              <span>PID: {status.pid}</span>
            </div>
          )}
        </div>

        {status.requestId && (
          <div className="flex items-center gap-1.5">
            <div className="flex items-center">
              <Hash className="h-3 w-3 mr-0.5 text-slate-500 dark:text-slate-400" />
              <span className="text-xs font-mono bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded text-slate-700 dark:text-slate-300">
                {status.requestId}
              </span>
            </div>

            <a
              href={url}
              onClick={(e) => {
                if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                  e.preventDefault();
                  onViewLog(status.requestId);
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

      {/* Timing Information */}
      <div className="border-b border-slate-200 dark:border-slate-700 pb-2">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex items-center">
            <Calendar className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Started">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                {formatTimestamp(status.startedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Clock className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Finished">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                {formatTimestamp(status.finishedAt)}
              </span>
            </LabeledItem>
          </div>

          <div className="flex items-center">
            <Timer className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <LabeledItem label="Duration">
              <span className="font-medium text-slate-700 dark:text-slate-300 text-xs">
                {calculateDuration()}
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

          {nodeStats.finished && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-green-500"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Success: {nodeStats.finished}
              </span>
            </div>
          )}

          {nodeStats.running && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-lime-500 animate-pulse"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Running: {nodeStats.running}
              </span>
            </div>
          )}

          {nodeStats.failed && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-red-500"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Failed: {nodeStats.failed}
              </span>
            </div>
          )}

          {nodeStats.not_started && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Not Started: {nodeStats.not_started}
              </span>
            </div>
          )}

          {nodeStats.skipped && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-slate-400"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Skipped: {nodeStats.skipped}
              </span>
            </div>
          )}

          {nodeStats.canceled && (
            <div className="flex items-center">
              <div className="h-2 w-2 mr-1 rounded-full bg-pink-400"></div>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                Canceled: {nodeStats.canceled}
              </span>
            </div>
          )}
        </div>

        {/* Progress bar */}
        {totalNodes > 0 && (
          <div className="mt-1.5 h-1.5 w-full bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
            {nodeStats.finished && (
              <div
                className="h-full bg-green-500 float-left"
                style={{ width: `${(nodeStats.finished / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStats.running && (
              <div
                className="h-full bg-lime-500 float-left animate-pulse"
                style={{ width: `${(nodeStats.running / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStats.failed && (
              <div
                className="h-full bg-red-500 float-left"
                style={{ width: `${(nodeStats.failed / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStats.skipped && (
              <div
                className="h-full bg-slate-400 float-left"
                style={{ width: `${(nodeStats.skipped / totalNodes) * 100}%` }}
              ></div>
            )}
            {nodeStats.canceled && (
              <div
                className="h-full bg-pink-400 float-left"
                style={{ width: `${(nodeStats.canceled / totalNodes) * 100}%` }}
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
        {status.status === Status.Cancelled && (
          <div className="mt-1.5 flex items-center text-xs text-slate-600 dark:text-slate-400">
            <StopCircle className="h-3 w-3 mr-1 text-pink-400" />
            <span>Execution was cancelled</span>
          </div>
        )}
      </div>

      {/* Parameters */}
      {status.params && (
        <div>
          <div className="flex items-center mb-1">
            <Terminal className="h-3.5 w-3.5 mr-1 text-slate-500 dark:text-slate-400" />
            <span className="text-xs font-semibold text-slate-700 dark:text-slate-300">
              Parameters
            </span>
          </div>
          <div className="p-1.5 bg-slate-100 dark:bg-slate-800 rounded-md font-medium text-xs text-slate-700 dark:text-slate-300 font-mono max-h-[120px] overflow-y-auto w-full">
            {status.params}
          </div>
        </div>
      )}
    </div>
  );
}

export default DAGStatusOverview;
