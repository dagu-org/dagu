/**
 * NodeStatusTableRow component renders a single row in the node status table.
 *
 * @module features/dags/components/dag-details
 */
import { TableCell } from '@/components/ui/table';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import dayjs from '@/lib/dayjs';
import { cn } from '@/lib/utils';
import { Code, FileText } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { NodeStatusChip } from '../common';

/**
 * Props for the NodeStatusTableRow component
 */
type Props = {
  /** Row number for display */
  rownum: number;
  /** Node data to display */
  node: components['schemas']['Node'];
  /** Request ID for log linking */
  requestId?: string;
  /** DAG name/fileId */
  name: string;
  /** Function to open log viewer */
  onViewLog?: (stepName: string, requestId: string) => void;
};

/**
 * Format timestamp for better readability
 */
const formatTimestamp = (timestamp: string | undefined) => {
  if (!timestamp || timestamp == '-') return '-';
  try {
    return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss Z');
  } catch {
    return timestamp;
  }
};

/**
 * Calculate duration between two timestamps
 * If endTime is not provided, calculate duration from startTime to now (for running tasks)
 */
const calculateDuration = (
  startTime: string | undefined,
  endTime: string | undefined
) => {
  if (!startTime) return '-';

  try {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : new Date().getTime();

    if (isNaN(start) || isNaN(end)) return '-';

    const durationMs = end - start;

    // Format duration
    if (durationMs < 0) return '-';
    if (durationMs < 1000) return `${durationMs}ms`;

    const seconds = Math.floor(durationMs / 1000);
    if (seconds < 60) return `${seconds}s`;

    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;

    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    return `${hours}h ${remainingMinutes}m ${remainingSeconds}s`;
  } catch {
    return '-';
  }
};

/**
 * NodeStatusTableRow displays information about a single node's execution status
 */
function NodeStatusTableRow({
  name,
  rownum,
  node,
  requestId,
  onViewLog,
}: Props) {
  // State to store the current duration for running tasks
  const [currentDuration, setCurrentDuration] = useState<string>('-');

  // Update duration every second for running tasks
  useEffect(() => {
    if (node.status === NodeStatus.Running && node.startedAt) {
      // Initial calculation
      setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));

      // Set up interval to update duration every second
      const intervalId = setInterval(() => {
        setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));
      }, 1000);

      // Clean up interval on unmount or when status changes
      return () => clearInterval(intervalId);
    } else {
      // For non-running tasks, calculate once
      setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));
    }
  }, [node.status, node.startedAt, node.finishedAt]);
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  searchParams.set('remoteNode', 'local');
  if (node.step) {
    searchParams.set('step', node.step.name);
  }
  if (requestId) {
    searchParams.set('requestId', requestId);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;

  // Extract arguments for display
  let args = '';
  if (node.step.args) {
    // Use uninterpolated args to avoid render issues with very long params
    args =
      node.step.cmdWithArgs?.replace(node.step.command || '', '').trimStart() ||
      '';
  }

  // Determine row highlight based on status
  const getRowHighlight = () => {
    switch (node.status) {
      case NodeStatus.Running:
        return 'bg-lime-50 dark:bg-lime-900/10';
      case NodeStatus.Failed:
        return 'bg-red-50 dark:bg-red-900/10';
      default:
        return '';
    }
  };

  return (
    <StyledTableRow
      className={cn(
        'hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors duration-200 h-auto',
        getRowHighlight()
      )}
    >
      <TableCell className="text-center py-2">
        <span className="font-semibold text-slate-700 dark:text-slate-300 text-xs">
          {rownum}
        </span>
      </TableCell>

      {/* Combined Step Name & Description */}
      <TableCell>
        <div className="space-y-0.5">
          <div className="text-sm font-semibold text-slate-800 dark:text-slate-200 text-wrap break-all">
            {node.step.name}
          </div>
          {node.step.description && (
            <div className="text-xs text-slate-500 dark:text-slate-400 leading-tight">
              {node.step.description}
            </div>
          )}
        </div>
      </TableCell>

      {/* Combined Command & Args */}
      <TableCell>
        <div className="space-y-1.5">
          {!node.step.command && node.step.cmdWithArgs ? (
            <div className="flex items-center gap-1.5 text-xs font-medium">
              <Code className="h-4 w-4 text-blue-500 dark:text-blue-400" />
              <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300">
                {node.step.cmdWithArgs}
              </span>
            </div>
          ) : null}

          {node.step.command && (
            <div className="flex items-center gap-1.5 text-xs font-medium">
              <Code className="h-4 w-4 text-blue-500 dark:text-blue-400" />
              <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300">
                {node.step.command}
              </span>
            </div>
          )}

          {args && (
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="pl-5 text-xs font-medium text-slate-500 dark:text-slate-400 truncate cursor-pointer leading-tight">
                  {args}
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <span className="max-w-[400px] break-all text-xs">{args}</span>
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      </TableCell>

      {/* Last Run & Duration */}
      <TableCell>
        <div className="space-y-0.5">
          <div className="font-medium text-slate-700 dark:text-slate-300 text-sm">
            {formatTimestamp(node.startedAt)}
          </div>
          {node.startedAt && (
            <div className="text-xs text-slate-500 dark:text-slate-400 flex items-center gap-1.5 leading-tight">
              <span className="font-medium flex items-center">
                Duration:
                {node.status === NodeStatus.Running && (
                  <span className="inline-block w-2 h-2 rounded-full bg-lime-500 ml-1.5 animate-pulse" />
                )}
              </span>
              {currentDuration}
            </div>
          )}
        </div>
      </TableCell>

      {/* Status */}
      <TableCell className="text-center">
        <NodeStatusChip status={node.status} size="sm">
          {node.statusText}
        </NodeStatusChip>
      </TableCell>

      {/* Error */}
      <TableCell>
        {node.error && (
          <div className="text-xs bg-red-50 dark:bg-red-900/10 border border-red-100 dark:border-red-800 rounded-md p-1.5 max-h-[80px] overflow-y-auto whitespace-pre-wrap break-words text-red-600 dark:text-red-400 leading-tight">
            {node.error}
          </div>
        )}
      </TableCell>

      {/* Log */}
      <TableCell className="text-center">
        {node.log ? (
          <a
            href={url}
            onClick={(e) => {
              // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, let the default behavior happen
              // which will open the link in a new tab
              if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                e.preventDefault();
                onViewLog(node.step.name, requestId || '');
              }
            }}
            className="inline-flex items-center justify-center p-2 transition-colors duration-200 rounded-md text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
            title="Click to view log (Cmd/Ctrl+Click to open in new tab)"
          >
            <span className="sr-only">View Log</span>
            <FileText className="h-4 w-4" />
          </a>
        ) : null}
      </TableCell>
    </StyledTableRow>
  );
}

export default NodeStatusTableRow;
