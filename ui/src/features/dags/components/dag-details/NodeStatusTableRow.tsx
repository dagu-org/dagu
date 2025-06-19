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
import {
  ChevronDown,
  ChevronRight,
  Code,
  FileText,
  GitBranch,
  PlayCircle,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, NodeStatus } from '../../../../api/v2/schema';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { NodeStatusChip } from '../common';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/ui/CustomDialog';
import { useClient } from '@/hooks/api';

/**
 * Props for the NodeStatusTableRow component
 */
type Props = {
  /** Row number for display */
  rownum: number;
  /** Node data to display */
  node: components['schemas']['Node'];
  /** DAG file name */
  name: string;
  /** Function to open log viewer */
  onViewLog?: (stepName: string, dagRunId: string) => void;
  /** Full dagRun details (optional) - used to determine if this is a child dagRun */
  dagRun: components['schemas']['DAGRunDetails'];
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
  onViewLog,
  dagRun,
  view = 'desktop',
}: Props) {
  const { dagRunId, name: dagName } = dagRun;
  const navigate = useNavigate();
  const client = useClient();
  // State to store the current duration for running tasks
  const [currentDuration, setCurrentDuration] = useState<string>('-');
  // State for expanding/collapsing parallel executions
  const [isExpanded, setIsExpanded] = useState(false);
  const [showDialog, setShowDialog] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Check if this is a child dagRun node
  const hasChildDAGRun =
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
  if (dagRunId) {
    searchParams.set('dagRunId', dagRunId);
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

  // Handle child dagRun navigation
  const handleChildDAGRunNavigation = (
    childIndex: number = 0,
    e?: React.MouseEvent
  ) => {
    if (hasChildDAGRun && node.children && node.children[childIndex]) {
      const childDAGRunId = node.children[childIndex].dagRunId;

      // Check if we're in a dagRun context or a DAG context
      // More reliable detection by checking the current URL path or the dagRun object
      const currentPath = window.location.pathname;
      const isModal = document.querySelector('.dagRun-modal-content') !== null;
      const isDAGRunContext =
        dagRun && (currentPath.startsWith('/dag-runs/') || isModal);

      if (isDAGRunContext) {
        // For dagRuns, navigate to /dag-runs/{root-dag-name}/{root-dag-run-id}?childDAGRunId=...
        const searchParams = new URLSearchParams();
        searchParams.set('childDAGRunId', childDAGRunId);

        // Determine root DAG information
        let rootDAGRunId: string;
        let rootDAGName: string;

        if (dagRun && dagRun.rootDAGRunId) {
          // If this is already a child dagRun, use its root information
          rootDAGRunId = dagRun.rootDAGRunId;
          rootDAGName = dagRun.rootDAGRunName;
          searchParams.set('dagRunId', dagRun.rootDAGRunId);
          searchParams.set('dagRunName', dagRun.rootDAGRunName);
        } else {
          // Otherwise, use the current dagRun as the root
          rootDAGRunId = dagRunId || '';
          rootDAGName = dagRun?.name || name;
          searchParams.set('dagRunId', dagRunId || '');
          searchParams.set('dagRunName', dagRun?.name || name);
        }

        searchParams.set('step', node.step.name);
        const url = `/dag-runs/${rootDAGName}/${rootDAGRunId}?${searchParams.toString()}`;

        // If Cmd/Ctrl key is pressed, open in new tab
        if (e && (e.metaKey || e.ctrlKey)) {
          window.open(url, '_blank');
        } else {
          navigate(url);
        }
      } else {
        // For DAGs, use the existing approach with query parameters
        const searchParams = new URLSearchParams();
        searchParams.set('childDAGRunId', childDAGRunId);

        // Use root dagRun information from the dagRun prop if available
        if (dagRun && dagRun.rootDAGRunId) {
          // If this is already a child dagRun, use its root information
          searchParams.set('dagRunId', dagRun.rootDAGRunId);
        } else {
          // Otherwise, use the current dagRun as the root
          searchParams.set('dagRunId', dagRunId || '');
        }

        // Add dagRunName parameter to avoid waiting for DAG details
        // Use the root dagRun name or current dagRun name
        if (dagRun) {
          searchParams.set('dagRunName', dagRun.rootDAGRunName);
        }

        searchParams.set('step', node.step.name);
        const url = `/dags/${name}?${searchParams.toString()}`;

        // If Cmd/Ctrl key is pressed, open in new tab
        if (e && (e.metaKey || e.ctrlKey)) {
          window.open(url, '_blank');
        } else {
          navigate(url);
        }
      }
    }
  };

  // Handle log viewing
  const handleViewLog = (e: React.MouseEvent<HTMLAnchorElement>) => {
    // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, let the default behavior happen
    // which will open the link in a new tab
    if (!(e.metaKey || e.ctrlKey) && onViewLog) {
      e.preventDefault();
      onViewLog(node.step.name, dagRunId || '');
    }
  };

  const handleRetry = async () => {
    setLoading(true);
    setError(null);
    try {
      await client.POST('/dag-runs/{name}/{dagRunId}/retry', {
        params: { path: { name: dagName, dagRunId } },
        body: { dagRunId, stepName: node.step.name },
      });
      setSuccess(true);
      setShowDialog(false);
    } catch (e: any) {
      setError(e?.data?.message || e.message || 'Retry failed');
    } finally {
      setLoading(false);
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
              {hasChildDAGRun && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex items-center text-blue-500 cursor-pointer">
                      <GitBranch className="h-4 w-4" />
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="text-xs">
                      Child DAG Run: {node.step.run}
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
            {hasChildDAGRun && (
              <>
                {node.children && node.children.length === 1 ? (
                  // Single child DAG run
                  <>
                    <div
                      className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline"
                      onClick={(e) => handleChildDAGRunNavigation(0, e)}
                      title="Click to view child DAG run (Cmd/Ctrl+Click to open in new tab)"
                    >
                      View Child DAG Run: {node.step.run}
                    </div>
                    {node.children[0]?.params && (
                      <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                        Parameters:{' '}
                        <span className="font-mono">
                          {node.children[0].params}
                        </span>
                      </div>
                    )}
                  </>
                ) : (
                  // Multiple child DAG runs (parallel execution)
                  <>
                    <div className="text-xs text-slate-600 dark:text-slate-400 mt-1">
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => setIsExpanded(!isExpanded)}
                          className="flex items-center gap-1 text-blue-500 dark:text-blue-400 font-medium hover:underline"
                        >
                          {isExpanded ? (
                            <ChevronDown className="h-3 w-3" />
                          ) : (
                            <ChevronRight className="h-3 w-3" />
                          )}
                          Parallel execution: {node.children?.length || 0} child
                          DAG runs
                        </button>
                      </div>
                      {isExpanded && node.children && (
                        <div className="mt-2 ml-4 space-y-1 border-l border-slate-200 dark:border-slate-700 pl-3">
                          {node.children.map((child, index) => (
                            <div key={child.dagRunId} className="py-1">
                              <div
                                className="text-xs text-blue-500 dark:text-blue-400 cursor-pointer hover:underline"
                                onClick={(e) =>
                                  handleChildDAGRunNavigation(index, e)
                                }
                                title="Click to view child DAG run (Cmd/Ctrl+Click to open in new tab)"
                              >
                                #{index + 1}: {node.step.run}
                              </div>
                              {child.params && (
                                <div className="text-xs text-slate-500 dark:text-slate-400 ml-4 font-mono">
                                  {child.params}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </>
                )}
              </>
            )}
          </div>
        </TableCell>

        {/* Combined Command & Args */}
        <TableCell>
          <div className="space-y-1.5">
            {!node.step.command && node.step.cmdWithArgs ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <div className="flex items-center gap-1.5 text-xs font-medium cursor-pointer">
                    <Code className="h-4 w-4 text-blue-500 dark:text-blue-400" />
                    <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300 break-all">
                      {node.step.cmdWithArgs}
                    </span>
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <pre className="max-w-[500px] whitespace-pre-wrap break-all text-xs">
                    {node.step.cmdWithArgs}
                  </pre>
                </TooltipContent>
              </Tooltip>
            ) : null}

            {node.step.command && (
              <div className="space-y-1">
                <div className="flex items-center gap-1.5 text-xs font-medium">
                  <Code className="h-4 w-4 text-blue-500 dark:text-blue-400" />
                  <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300 break-all">
                    {node.step.command}
                  </span>
                </div>

                {node.step.args && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className="pl-5 text-xs font-medium text-slate-500 dark:text-slate-400 cursor-pointer leading-tight">
                        <span className="break-all whitespace-pre-wrap">
                          {Array.isArray(node.step.args)
                            ? node.step.args.join(' ')
                            : node.step.args}
                        </span>
                      </div>
                    </TooltipTrigger>
                    <TooltipContent>
                      <pre className="max-w-[500px] whitespace-pre-wrap break-all text-xs">
                        {Array.isArray(node.step.args)
                          ? node.step.args.join(' ')
                          : node.step.args}
                      </pre>
                    </TooltipContent>
                  </Tooltip>
                )}
              </div>
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

        {/* Error / Logs */}
        <TableCell>
          <div className="space-y-1.5">
            {/* Logs */}
            {(node.stdout || node.stderr) && (
              <div className="flex items-center gap-1.5">
                {/* Single log file - show simple button */}
                {(node.stdout && !node.stderr) ||
                  (!node.stdout && node.stderr) ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <a
                        href={node.stderr ? `${url}&stream=stderr` : url}
                        onClick={
                          node.stderr
                            ? (e) => {
                              if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                                e.preventDefault();
                                onViewLog(
                                  `${node.step.name}_stderr`,
                                  dagRunId || ''
                                );
                              }
                            }
                            : handleViewLog
                        }
                        className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium transition-colors duration-200 rounded cursor-pointer text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700"
                        title={`Click to view ${node.stderr ? 'stderr' : 'stdout'} log (Cmd/Ctrl+Click to open in new tab)`}
                      >
                        <FileText className="h-3 w-3" />
                        {node.stderr ? 'stderr' : 'stdout'}
                      </a>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span className="text-xs">
                        {node.stderr ? 'Error' : 'Output'} Log
                      </span>
                    </TooltipContent>
                  </Tooltip>
                ) : (
                  /* Both stdout and stderr - show combined button with split design */
                  <div className="inline-flex rounded overflow-hidden">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <a
                          href={url}
                          onClick={handleViewLog}
                          className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium transition-colors duration-200 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer border-r border-slate-200 dark:border-slate-700"
                          title="Click to view stdout log (Cmd/Ctrl+Click to open in new tab)"
                        >
                          <FileText className="h-3 w-3" />
                          out
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
                              onViewLog(
                                `${node.step.name}_stderr`,
                                dagRunId || ''
                              );
                            }
                          }}
                          className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium transition-colors duration-200 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer"
                          title="Click to view stderr log (Cmd/Ctrl+Click to open in new tab)"
                        >
                          <FileText className="h-3 w-3" />
                          err
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

            {/* Errors - Simplified */}
            {node.error && (
              <div className="text-xs text-red-600 dark:text-red-400 leading-relaxed whitespace-normal break-words">
                {node.error}
              </div>
            )}
            {node.step.preconditions?.some((cond) => cond.error) && (
              <div className="text-xs text-amber-600 dark:text-amber-400 leading-relaxed">
                Precondition unmet
              </div>
            )}
          </div>
        </TableCell>

        <TableCell className="text-center">
          <button
            className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700"
            title="Retry from this step"
            onClick={() => setShowDialog(true)}
            disabled={loading}
          >
            <PlayCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
          </button>
          <Dialog open={showDialog} onOpenChange={setShowDialog}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Retry from this step?</DialogTitle>
              </DialogHeader>
              <div className="py-2 text-sm">
                This will re-execute <b>{node.step.name}</b> and all downstream steps. Are you sure?
                {error && <div className="text-red-500 mt-2">{error}</div>}
                {success && <div className="text-green-600 mt-2">Retry started!</div>}
              </div>
              <DialogFooter>
                <button
                  className="px-3 py-1 rounded bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 mr-2"
                  onClick={() => setShowDialog(false)}
                  disabled={loading}
                >
                  Cancel
                </button>
                <button
                  className="px-3 py-1 rounded bg-green-600 text-white hover:bg-green-700 disabled:opacity-50"
                  onClick={handleRetry}
                  disabled={loading}
                >
                  {loading ? 'Retrying...' : 'Retry'}
                </button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </TableCell>
      </StyledTableRow>
    );
  }

  // Render mobile view (card)
  return (
    <div
      className={cn(
        'p-4 rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-zinc-900 shadow-sm hover:shadow-md transition-shadow duration-200',
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
            {hasChildDAGRun && (
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

      {/* Child dagRun link */}
      {hasChildDAGRun && (
        <>
          {node.children && node.children.length === 1 ? (
            // Single child DAG run
            <>
              <div
                className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline mb-1"
                onClick={(e) => handleChildDAGRunNavigation(0, e)}
              >
                View Child DAG Run: {node.step.run}
              </div>
              {node.children[0]?.params && (
                <div className="text-xs text-slate-500 dark:text-slate-400 mb-3">
                  Parameters:{' '}
                  <span className="font-mono">{node.children[0].params}</span>
                </div>
              )}
            </>
          ) : (
            // Multiple child DAG runs (parallel execution)
            <div className="mb-3">
              <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="flex items-center gap-1 text-xs text-blue-500 dark:text-blue-400 font-medium hover:underline"
              >
                {isExpanded ? (
                  <ChevronDown className="h-3 w-3" />
                ) : (
                  <ChevronRight className="h-3 w-3" />
                )}
                Parallel execution: {node.children?.length || 0} child DAG runs
              </button>
              {isExpanded && node.children && (
                <div className="mt-2 ml-4 space-y-1 border-l border-slate-200 dark:border-slate-700 pl-3">
                  {node.children.map((child, index) => (
                    <div key={child.dagRunId} className="py-1">
                      <div
                        className="text-xs text-blue-500 dark:text-blue-400 cursor-pointer hover:underline"
                        onClick={(e) => handleChildDAGRunNavigation(index, e)}
                      >
                        #{index + 1}: {node.step.run}
                      </div>
                      {child.params && (
                        <div className="text-xs text-slate-500 dark:text-slate-400 ml-4 font-mono">
                          {child.params}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </>
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
              <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300 break-all whitespace-pre-wrap">
                {node.step.cmdWithArgs}
              </span>
            </div>
          ) : null}

          {node.step.command && (
            <div className="space-y-1">
              <div className="flex items-center gap-1.5 text-xs font-medium">
                <Code className="h-4 w-4 text-blue-500 dark:text-blue-400" />
                <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-1.5 py-0.5 text-slate-700 dark:text-slate-300 break-all whitespace-pre-wrap">
                  {node.step.command}
                </span>
              </div>

              {node.step.args && (
                <div className="pl-5 text-xs font-medium text-slate-500 dark:text-slate-400 leading-tight">
                  <span className="break-all whitespace-pre-wrap">
                    {Array.isArray(node.step.args)
                      ? node.step.args.join(' ')
                      : node.step.args}
                  </span>
                </div>
              )}
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
              onClick={
                node.stderr
                  ? (e) => {
                    if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                      e.preventDefault();
                      onViewLog(`${node.step.name}_stderr`, dagRunId || '');
                    }
                  }
                  : handleViewLog
              }
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 rounded-md cursor-pointer text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-700"
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
                    onViewLog(`${node.step.name}_stderr`, dagRunId || '');
                  }
                }}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors duration-200 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
                title="Click to view stderr log (Cmd/Ctrl+Click to open in new tab)"
              >
                <FileText className="h-3.5 w-3.5" />
                stderr
              </a>
            </div>
          )}
        </div>
      )}

      <div className="flex justify-end mt-4">
        <button
          className="p-2 rounded-full hover:bg-slate-200 dark:hover:bg-slate-700"
          title="Retry from this step"
          onClick={() => setShowDialog(true)}
          disabled={loading}
        >
          <PlayCircle className="h-6 w-6 text-green-600 dark:text-green-400" />
        </button>
        <Dialog open={showDialog} onOpenChange={setShowDialog}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Retry from this step?</DialogTitle>
            </DialogHeader>
            <div className="py-2 text-sm">
              This will re-execute <b>{node.step.name}</b> and all downstream steps. Are you sure?
              {error && <div className="text-red-500 mt-2">{error}</div>}
              {success && <div className="text-green-600 mt-2">Retry started!</div>}
            </div>
            <DialogFooter>
              <button
                className="px-3 py-1 rounded bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 mr-2"
                onClick={() => setShowDialog(false)}
                disabled={loading}
              >
                Cancel
              </button>
              <button
                className="px-3 py-1 rounded bg-green-600 text-white hover:bg-green-700 disabled:opacity-50"
                onClick={handleRetry}
                disabled={loading}
              >
                {loading ? 'Retrying...' : 'Retry'}
              </button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );
}

export default NodeStatusTableRow;
