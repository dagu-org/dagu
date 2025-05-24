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
import { Code, FileText, GitBranch } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
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
  /** Workflow ID for log linking */
  workflowId?: string;
  /** DAG name or file name */
  name: string;
  /** Function to open log viewer */
  onViewLog?: (stepName: string, workflowId: string) => void;
  /** Full workflow details (optional) - used to determine if this is a child workflow */
  workflow?: components['schemas']['WorkflowDetails'];
  /** View mode: desktop or mobile */
  view?: 'desktop' | 'mobile';
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
  workflowId,
  onViewLog,
  workflow,
  view = 'desktop',
}: Props) {
  const navigate = useNavigate();
  // State to store the current duration for running tasks
  const [currentDuration, setCurrentDuration] = useState<string>('-');

  // Check if this is a child workflow node
  const hasChildWorkflow =
    !!node.step.run && node.children && node.children.length > 0;

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
  if (workflowId) {
    searchParams.set('workflowId', workflowId);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;

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

  // Handle child workflow navigation
  const handleChildWorkflowNavigation = () => {
    if (hasChildWorkflow && node.children && node.children[0]) {
      const childWorkflowId = node.children[0].workflowId;

      // Check if we're in a workflow context or a DAG context
      // More reliable detection by checking the current URL path or the workflow object
      const currentPath = window.location.pathname;
      const isModal =
        document.querySelector('.workflow-modal-content') !== null;
      const isWorkflowContext =
        workflow && (currentPath.startsWith('/workflows/') || isModal);

      if (isWorkflowContext) {
        // For workflows, use query parameters to navigate to the workflow details page
        const searchParams = new URLSearchParams();
        searchParams.set('childWorkflowId', childWorkflowId);

        // Use root workflow information from the workflow prop if available
        if (workflow && workflow.rootWorkflowId) {
          // If this is already a child workflow, use its root information
          searchParams.set('workflowId', workflow.rootWorkflowId);
          searchParams.set('workflowName', workflow.rootWorkflowName);
        } else {
          // Otherwise, use the current workflow as the root
          searchParams.set('workflowId', workflowId || '');
          searchParams.set('workflowName', workflow?.name || name);
        }

        searchParams.set('step', node.step.name);
        navigate(
          `/workflows/${workflow?.name || name}?${searchParams.toString()}`
        );
      } else {
        // For DAGs, use the existing approach with query parameters
        const searchParams = new URLSearchParams();
        searchParams.set('childWorkflowId', childWorkflowId);

        // Use root workflow information from the workflow prop if available
        if (workflow && workflow.rootWorkflowId) {
          // If this is already a child workflow, use its root information
          searchParams.set('workflowId', workflow.rootWorkflowId);
        } else {
          // Otherwise, use the current workflow as the root
          searchParams.set('workflowId', workflowId || '');
        }

        // Add workflowName parameter to avoid waiting for DAG details
        // Use the root workflow name or current workflow name
        if (workflow) {
          searchParams.set('workflowName', workflow.rootWorkflowName);
        }

        searchParams.set('step', node.step.name);
        navigate(`/dags/${name}?${searchParams.toString()}`);
      }
    }
  };

  // Handle log viewing
  const handleViewLog = (e: React.MouseEvent<HTMLAnchorElement>) => {
    // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, let the default behavior happen
    // which will open the link in a new tab
    if (!(e.metaKey || e.ctrlKey) && onViewLog) {
      e.preventDefault();
      onViewLog(node.step.name, workflowId || '');
    }
  };

  // Render desktop view (table row)
  if (view === 'desktop') {
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
            <div className="text-sm font-semibold text-slate-800 dark:text-slate-200 text-wrap break-all flex items-center gap-1.5">
              {node.step.name}
              {hasChildWorkflow && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex items-center text-blue-500 cursor-pointer">
                      <GitBranch className="h-4 w-4" />
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="text-xs">
                      Child Workflow: {node.step.run}
                    </span>
                  </TooltipContent>
                </Tooltip>
              )}
            </div>
            {node.step.description && (
              <div className="text-xs text-slate-500 dark:text-slate-400 leading-tight">
                {node.step.description}
              </div>
            )}
            {hasChildWorkflow && (
              <div
                className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline"
                onClick={handleChildWorkflowNavigation}
              >
                View Child Workflow: {node.step.run}
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

            {node.step.args && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <div className="pl-5 text-xs font-medium text-slate-500 dark:text-slate-400 truncate cursor-pointer leading-tight">
                    {node.step.args}
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <span className="max-w-[400px] break-all text-xs">
                    {node.step.args}
                  </span>
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
            {node.statusLabel}
          </NodeStatusChip>
        </TableCell>

        {/* Error */}
        <TableCell>
          {node.error && (
            <div className="text-xs bg-red-50 dark:bg-red-900/10 border border-red-100 dark:border-red-800 rounded-md p-1.5 max-h-[80px] overflow-y-auto whitespace-pre-wrap break-words text-red-600 dark:text-red-400 leading-tight">
              {node.error}
            </div>
          )}
          {node.step.preconditions?.some((cond) => cond.error) && (
            <div className="mt-2">
              <div className="text-xs font-medium text-amber-600 dark:text-amber-400 mb-1">
                Precondition Unmet:
              </div>
              {node.step.preconditions
                .filter((cond) => cond.error)
                .map((cond, idx) => (
                  <div
                    key={idx}
                    className="text-xs bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-800 rounded-md p-1.5 mb-1 whitespace-pre-wrap break-words text-amber-600 dark:text-amber-400 leading-tight"
                  >
                    <div className="font-medium">
                      Condition: {cond.condition}
                    </div>
                    <div>Expected: {cond.expected}</div>
                    <div>Error: {cond.error}</div>
                  </div>
                ))}
            </div>
          )}
        </TableCell>

        {/* Log */}
        <TableCell className="text-center">
          {(node.stdout || node.stderr) && (
            <div className="relative inline-flex">
              {/* Single log file - show simple button */}
              {(node.stdout && !node.stderr) || (!node.stdout && node.stderr) ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <a
                      href={node.stderr ? `${url}&stream=stderr` : url}
                      onClick={node.stderr ? (e) => {
                        if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                          e.preventDefault();
                          onViewLog(`${node.step.name}_stderr`, workflowId || '');
                        }
                      } : handleViewLog}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 rounded-md cursor-pointer ${
                        node.stderr 
                          ? 'text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 hover:bg-red-50 dark:hover:bg-red-900/20 border border-red-200 dark:border-red-800'
                          : 'text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-700'
                      }`}
                      title={`Click to view ${node.stderr ? 'stderr' : 'stdout'} log (Cmd/Ctrl+Click to open in new tab)`}
                    >
                      <FileText className="h-3.5 w-3.5" />
                      {node.stderr ? 'stderr' : 'stdout'}
                    </a>
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="text-xs">{node.stderr ? 'Error' : 'Output'} Log</span>
                  </TooltipContent>
                </Tooltip>
              ) : (
                /* Both stdout and stderr - show combined button with split design */
                <div className="inline-flex rounded-md border border-slate-200 dark:border-slate-700 overflow-hidden">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <a
                        href={url}
                        onClick={handleViewLog}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer border-r border-slate-200 dark:border-slate-700"
                        title="Click to view stdout log (Cmd/Ctrl+Click to open in new tab)"
                      >
                        <FileText className="h-3.5 w-3.5" />
                        stdout
                      </a>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span className="text-xs">Output Log</span>
                    </TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <a
                        href={`${url}&stream=stderr`}
                        onClick={(e) => {
                          if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                            e.preventDefault();
                            onViewLog(`${node.step.name}_stderr`, workflowId || '');
                          }
                        }}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 hover:bg-red-50 dark:hover:bg-red-900/20 cursor-pointer"
                        title="Click to view stderr log (Cmd/Ctrl+Click to open in new tab)"
                      >
                        <FileText className="h-3.5 w-3.5" />
                        stderr
                      </a>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span className="text-xs">Error Log</span>
                    </TooltipContent>
                  </Tooltip>
                </div>
              )}
            </div>
          )}
        </TableCell>
      </StyledTableRow>
    );
  }

  // Render mobile view (card)
  return (
    <div
      className={cn(
        'p-4 rounded-lg border border-slate-200 dark:border-slate-700',
        getRowHighlight()
      )}
    >
      {/* Header with number and status */}
      <div className="flex justify-between items-center mb-3">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-slate-700 dark:text-slate-300 text-sm bg-slate-100 dark:bg-slate-800 rounded-full w-6 h-6 flex items-center justify-center">
            {rownum}
          </span>
          <h3 className="font-semibold text-slate-800 dark:text-slate-200">
            {node.step.name}
            {hasChildWorkflow && (
              <span className="inline-flex items-center text-blue-500 ml-1.5">
                <GitBranch className="h-4 w-4" />
              </span>
            )}
          </h3>
        </div>
        <NodeStatusChip status={node.status} size="sm">
          {node.statusLabel}
        </NodeStatusChip>
      </div>

      {/* Description */}
      {node.step.description && (
        <div className="text-xs text-slate-500 dark:text-slate-400 mb-3">
          {node.step.description}
        </div>
      )}

      {/* Child workflow link */}
      {hasChildWorkflow && (
        <div
          className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline mb-3"
          onClick={handleChildWorkflowNavigation}
        >
          View Child Workflow: {node.step.run}
        </div>
      )}

      {/* Command section */}
      <div className="mb-3">
        <div className="text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">
          Command:
        </div>
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

          {node.step.args && (
            <div className="pl-5 text-xs font-medium text-slate-500 dark:text-slate-400 break-words leading-tight">
              {node.step.args}
            </div>
          )}
        </div>
      </div>

      {/* Timing section */}
      <div className="mb-3">
        <div className="text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">
          Timing:
        </div>
        <div className="space-y-0.5">
          <div className="text-xs text-slate-600 dark:text-slate-400">
            Started: {formatTimestamp(node.startedAt)}
          </div>
          {node.startedAt && (
            <div className="text-xs text-slate-600 dark:text-slate-400 flex items-center gap-1.5">
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
      </div>

      {/* Error section */}
      {(node.error || node.step.preconditions?.some((cond) => cond.error)) && (
        <div className="mb-3">
          <div className="text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">
            Errors:
          </div>

          {node.error && (
            <div className="text-xs bg-red-50 dark:bg-red-900/10 border border-red-100 dark:border-red-800 rounded-md p-1.5 mb-2 whitespace-pre-wrap break-words text-red-600 dark:text-red-400 leading-tight">
              {node.error}
            </div>
          )}

          {node.step.preconditions?.some((cond) => cond.error) && (
            <div>
              <div className="text-xs font-medium text-amber-600 dark:text-amber-400 mb-1">
                Precondition Unmet:
              </div>
              {node.step.preconditions
                .filter((cond) => cond.error)
                .map((cond, idx) => (
                  <div
                    key={idx}
                    className="text-xs bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-800 rounded-md p-1.5 mb-1 whitespace-pre-wrap break-words text-amber-600 dark:text-amber-400 leading-tight"
                  >
                    <div className="font-medium">
                      Condition: {cond.condition}
                    </div>
                    <div>Expected: {cond.expected}</div>
                    <div>Error: {cond.error}</div>
                  </div>
                ))}
            </div>
          )}
        </div>
      )}

      {/* Log buttons */}
      {(node.stdout || node.stderr) && (
        <div className="flex justify-end">
          {/* Single log file - show simple button */}
          {(node.stdout && !node.stderr) || (!node.stdout && node.stderr) ? (
            <a
              href={node.stderr ? `${url}&stream=stderr` : url}
              onClick={node.stderr ? (e) => {
                if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                  e.preventDefault();
                  onViewLog(`${node.step.name}_stderr`, workflowId || '');
                }
              } : handleViewLog}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 rounded-md cursor-pointer ${
                node.stderr 
                  ? 'text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 hover:bg-red-50 dark:hover:bg-red-900/20 border border-red-200 dark:border-red-800'
                  : 'text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-700'
              }`}
              title={`Click to view ${node.stderr ? 'stderr' : 'stdout'} log (Cmd/Ctrl+Click to open in new tab)`}
            >
              <FileText className="h-3.5 w-3.5" />
              {node.stderr ? 'stderr' : 'stdout'}
            </a>
          ) : (
            /* Both stdout and stderr - show combined button with split design */
            <div className="inline-flex rounded-md border border-slate-200 dark:border-slate-700 overflow-hidden">
              <a
                href={url}
                onClick={handleViewLog}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer border-r border-slate-200 dark:border-slate-700"
                title="Click to view stdout log (Cmd/Ctrl+Click to open in new tab)"
              >
                <FileText className="h-3.5 w-3.5" />
                stdout
              </a>
              <a
                href={`${url}&stream=stderr`}
                onClick={(e) => {
                  if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                    e.preventDefault();
                    onViewLog(`${node.step.name}_stderr`, workflowId || '');
                  }
                }}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 hover:bg-red-50 dark:hover:bg-red-900/20 cursor-pointer"
                title="Click to view stderr log (Cmd/Ctrl+Click to open in new tab)"
              >
                <FileText className="h-3.5 w-3.5" />
                stderr
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default NodeStatusTableRow;
