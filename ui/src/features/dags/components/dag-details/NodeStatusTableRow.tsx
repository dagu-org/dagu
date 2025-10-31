/**
 * NodeStatusTableRow component renders a single row in the node status table.
 *
 * @module features/dags/components/dag-details
 */
import { CommandDisplay } from '@/components/ui/command-display';
import { TableCell } from '@/components/ui/table';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient, useQuery } from '@/hooks/api';
import dayjs from '@/lib/dayjs';
import { cn } from '@/lib/utils';
import { SubDAGRunsList } from './SubDAGRunsList';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import {
  ChevronDown,
  ChevronRight,
  Code,
  FileText,
  GitBranch,
  PlayCircle,
  RefreshCw,
} from 'lucide-react';
import { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, NodeStatus, Stream } from '../../../../api/v2/schema';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { NodeStatusChip } from '../common';
import StatusUpdateModal from '../dag-execution/StatusUpdateModal';

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
  onViewLog?: (
    stepName: string,
    dagRunId: string,
    node?: components['schemas']['Node']
  ) => void;
  /** Full dagRun details (optional) - used to determine if this is a sub dagRun */
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
 * ANSI color codes regex for stripping
 */
const ANSI_CODES_REGEX = [
  '[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)',
  '(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))',
].join('|');

/**
 * Simple inline log viewer - no controls, just logs
 */
function InlineLogViewer({
  dagName,
  dagRunId,
  stepName,
  stream,
  dagRun,
}: {
  dagName: string;
  dagRunId: string;
  stepName: string;
  stream: components['schemas']['Stream'];
  dagRun?: components['schemas']['DAGRunDetails'];
}) {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Determine if this is a sub DAG run
  const isSubDAGRun =
    dagRun && dagRun.rootDAGRunId && dagRun.rootDAGRunId !== dagRun.dagRunId;

  // Determine the API endpoint
  const apiEndpoint = isSubDAGRun
    ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/log'
    : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/log';

  // Prepare path parameters
  const pathParams = isSubDAGRun
    ? {
        name: dagRun.rootDAGRunName,
        dagRunId: dagRun.rootDAGRunId,
        subDAGRunId: dagRun.dagRunId,
        stepName,
      }
    : {
        name: dagName,
        dagRunId,
        stepName,
      };

  // Fetch last 100 lines
  const { data, isLoading } = useQuery(
    apiEndpoint,
    {
      params: {
        query: {
          remoteNode,
          stream,
          tail: 100,
        },
        path: pathParams,
      },
    },
    {
      refreshInterval: 2000, // Auto-refresh every 2s
      revalidateOnFocus: false,
    }
  );

  // Process log content
  const content =
    data?.content?.replace(new RegExp(ANSI_CODES_REGEX, 'g'), '') || '';
  const lines = content ? content.split('\n') : [];
  const totalLines = data?.totalLines || 0;
  const lineCount = data?.lineCount || 0;

  return (
    <div className="bg-zinc-900 rounded overflow-hidden">
      {isLoading && !data ? (
        <div className="text-zinc-400 text-xs py-4 px-3">Loading logs...</div>
      ) : lines.length === 0 ? (
        <div className="text-zinc-400 text-xs py-4 px-3">
          &lt;No log output&gt;
        </div>
      ) : (
        <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
          <pre className="font-mono text-[11px] text-zinc-100 p-2">
            {lines.map((line, index) => {
              const lineNumber = totalLines - lineCount + index + 1;
              return (
                <div key={index} className="flex hover:bg-zinc-800 px-1 py-0.5">
                  <span className="text-zinc-500 mr-3 select-none w-12 text-right flex-shrink-0">
                    {lineNumber}
                  </span>
                  <span className="whitespace-pre-wrap break-all flex-grow">
                    {line || ' '}
                  </span>
                </div>
              );
            })}
          </pre>
        </div>
      )}
    </div>
  );
}

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
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  // State to store the current duration for running tasks
  const [currentDuration, setCurrentDuration] = useState<string>('-');
  // State for expanding/collapsing parallel executions
  const [isExpanded, setIsExpanded] = useState(false);
  const [showDialog, setShowDialog] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  // State for inline log expansion
  const [isLogExpanded, setIsLogExpanded] = useState(false);
  const [activeLogTab, setActiveLogTab] = useState<'stdout' | 'stderr'>(
    'stdout'
  );
  // State for status update modal
  const [showStatusModal, setShowStatusModal] = useState(false);
  // Check if this is a sub dagRun node
  // Include both regular and repeated sub runs
  const allSubRuns = [
    ...(node.subRuns || []),
    ...(node.subRunsRepeated || []),
  ];
  const subDagName = node.step.call;
  const hasSubDAGRun = !!subDagName && allSubRuns.length > 0;

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
  searchParams.set('remoteNode', remoteNode);
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

  // Handle sub dagRun navigation
  const handleSubDAGRunNavigation = (
    subRunIndex: number = 0,
    e?: React.MouseEvent
  ) => {
    if (hasSubDAGRun && allSubRuns[subRunIndex]) {
      const subDAGRunId = allSubRuns[subRunIndex].dagRunId;

      // Check if we're in a dagRun context or a DAG context
      // More reliable detection by checking the current URL path or the dagRun object
      const currentPath = window.location.pathname;
      const isModal = document.querySelector('.dagRun-modal-content') !== null;
      const isDAGRunContext =
        dagRun && (currentPath.startsWith('/dag-runs/') || isModal);

      if (isDAGRunContext) {
        // For dagRuns, navigate to /dag-runs/{root-dag-name}/{root-dag-run-id}?subDAGRunId=...
        const searchParams = new URLSearchParams();
        searchParams.set('subDAGRunId', subDAGRunId);

        // Determine root DAG information
        let rootDAGRunId: string;
        let rootDAGName: string;

        if (dagRun && dagRun.rootDAGRunId) {
          // If this is already a sub dagRun, use its root information
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
        searchParams.set('subDAGRunId', subDAGRunId);

        // Use root dagRun information from the dagRun prop if available
        if (dagRun && dagRun.rootDAGRunId) {
          // If this is already a sub dagRun, use its root information
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
      onViewLog(node.step.name, dagRunId || '', node);
    }
  };

  const handleRetry = async () => {
    setLoading(true);
    setError(null);
    try {
      await client.POST('/dag-runs/{name}/{dagRunId}/retry', {
        params: {
          path: { name: dagName, dagRunId },
          query: { remoteNode },
        },
        body: { dagRunId, stepName: node.step.name },
      });
      setSuccess(true);
      setShowDialog(false);
    } catch (e) {
      const error = e as { data?: { message?: string }; message?: string };
      setError(error?.data?.message || error.message || 'Retry failed');
    } finally {
      setLoading(false);
    }
  };

  // Handle status update
  const handleStatusUpdate = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    // Check if this is a sub DAG-run
    const isSubDAGRun =
      dagRun.rootDAGRunId &&
      dagRun.rootDAGRunName &&
      dagRun.rootDAGRunId !== dagRun.dagRunId;

    // Define path parameters
    const pathParams = {
      name: isSubDAGRun ? dagRun.rootDAGRunName : dagName,
      dagRunId: isSubDAGRun ? dagRun.rootDAGRunId : dagRunId || '',
      stepName: step.name,
      ...(isSubDAGRun ? { subDAGRunId: dagRun.dagRunId } : {}),
    };

    // Use the appropriate endpoint
    const endpoint = isSubDAGRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/status'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/status';

    const { error } = await client.PATCH(endpoint, {
      params: {
        path: pathParams,
        query: {
          remoteNode,
        },
      },
      body: {
        status,
      },
    });

    if (error) {
      alert(error.message || 'An error occurred');
      return;
    }

    setShowStatusModal(false);
  };

  // Determine if logs are available
  const hasStdout = !!node.stdout;
  const hasStderr = !!node.stderr;
  const hasLogs = hasStdout || hasStderr;

  // Determine which stream to show based on active tab
  const currentStream: components['schemas']['Stream'] =
    activeLogTab === 'stderr' && hasStderr ? Stream.stderr : Stream.stdout;

  // Render desktop view (table row)
  if (view === 'desktop') {
    return (
      <>
        <StyledTableRow
          className={cn(
            'hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors duration-200 h-auto cursor-pointer',
            getRowHighlight()
          )}
          onClick={() => {
            if (hasLogs) {
              setIsLogExpanded(!isLogExpanded);
              // Set default tab based on what's available
              if (!isLogExpanded) {
                setActiveLogTab(hasStdout ? 'stdout' : 'stderr');
              }
            }
          }}
        >
          <TableCell className="text-center py-2">
            <div className="flex items-center justify-center gap-2">
              {hasLogs && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setIsLogExpanded(!isLogExpanded);
                    if (!isLogExpanded) {
                      setActiveLogTab(hasStdout ? 'stdout' : 'stderr');
                    }
                  }}
                  className="text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
                >
                  {isLogExpanded ? (
                    <ChevronDown className="h-4 w-4" />
                  ) : (
                    <ChevronRight className="h-4 w-4" />
                  )}
                </button>
              )}
              <span className="font-semibold text-slate-700 dark:text-slate-300 text-xs">
                {rownum}
              </span>
            </div>
          </TableCell>

          {/* Combined Step Name & Description */}
          <TableCell>
            <div className="space-y-0.5">
              <div className="text-sm font-semibold text-slate-800 dark:text-slate-200 text-wrap break-all flex items-center gap-1.5">
                {node.step.name}
                {hasSubDAGRun && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="inline-flex items-center text-blue-500 cursor-pointer">
                        <GitBranch className="h-4 w-4" />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span className="text-xs">
                        Sub DAG Run: {subDagName}
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

              {/* Repeat Policy */}
              {node.step.repeatPolicy?.repeat && (
                <div className="flex items-start gap-1 mt-1">
                  <span
                    className={`inline-flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider px-1.5 py-0.5 rounded ${
                      node.step.repeatPolicy.repeat === 'while'
                        ? 'bg-cyan-100 dark:bg-cyan-900/30 text-cyan-700 dark:text-cyan-300'
                        : node.step.repeatPolicy.repeat === 'until'
                          ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300'
                          : 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                    }`}
                  >
                    <RefreshCw className="h-2.5 w-2.5" />
                    {node.step.repeatPolicy.repeat === 'while'
                      ? 'WHILE'
                      : node.step.repeatPolicy.repeat === 'until'
                        ? 'UNTIL'
                        : 'REPEAT'}
                    {node.step.repeatPolicy.interval && (
                      <span className="opacity-75">
                        {node.step.repeatPolicy.interval}s
                      </span>
                    )}
                    {node.step.repeatPolicy.limit && (
                      <span className="opacity-75">
                        ×{node.step.repeatPolicy.limit}
                      </span>
                    )}
                  </span>

                  {node.step.repeatPolicy.condition && (
                    <span className="text-[10px] text-slate-600 dark:text-slate-400 font-mono">
                      {node.step.repeatPolicy.condition.condition}
                      {node.step.repeatPolicy.condition.expected && (
                        <span className="text-emerald-600 dark:text-emerald-400">
                          ={node.step.repeatPolicy.condition.expected}
                        </span>
                      )}
                    </span>
                  )}

                  {node.step.repeatPolicy.exitCode &&
                    node.step.repeatPolicy.exitCode.length > 0 && (
                      <span className="text-[10px] text-slate-600 dark:text-slate-400 font-mono">
                        exit:[{node.step.repeatPolicy.exitCode.join(',')}]
                      </span>
                    )}
                </div>
              )}

              {hasSubDAGRun && (
                <SubDAGRunsList
                  dagName={dagName}
                  dagRunId={dagRunId || ''}
                  subDagName={subDagName || ''}
                  allSubRuns={allSubRuns}
                  isExpanded={isExpanded}
                  onToggleExpand={() => setIsExpanded(!isExpanded)}
                  onNavigate={handleSubDAGRunNavigation}
                />
              )}
            </div>
          </TableCell>

          {/* Combined Command & Args */}
          <TableCell>
            {(node.step.command || node.step.cmdWithArgs) && (
              <CommandDisplay
                command={node.step.command || node.step.cmdWithArgs || ''}
                args={node.step.command ? node.step.args : undefined}
                icon="code"
                maxLength={50}
              />
            )}
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
            <div
              onClick={(e) => {
                e.stopPropagation();
                setShowStatusModal(true);
              }}
              className="inline-block cursor-pointer"
              title="Click to update status"
            >
              <NodeStatusChip status={node.status} size="sm">
                {node.statusLabel}
              </NodeStatusChip>
            </div>
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
                                  e.stopPropagation();
                                  if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                                    e.preventDefault();
                                    onViewLog(
                                      `${node.step.name}_stderr`,
                                      dagRunId || ''
                                    );
                                  }
                                }
                              : (e) => {
                                  e.stopPropagation();
                                  handleViewLog(e);
                                }
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
                            onClick={(e) => {
                              e.stopPropagation();
                              handleViewLog(e);
                            }}
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
                              e.stopPropagation();
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
          {dagRunId && (
            <TableCell className="text-center">
              <button
                className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700"
                title="Retry from this step"
                onClick={(e) => {
                  e.stopPropagation();
                  setShowDialog(true);
                }}
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
                    This will re-execute <b>{node.step.name}</b>. Are you sure?
                    {error && <div className="text-red-500 mt-2">{error}</div>}
                    {success && (
                      <div className="text-green-600 mt-2">Retry started!</div>
                    )}
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
          )}
        </StyledTableRow>

        {/* Inline log viewer row - spans entire table width */}
        {isLogExpanded && hasLogs && (
          <StyledTableRow className="bg-zinc-50 dark:bg-zinc-900">
            <TableCell colSpan={dagRunId ? 7 : 6} className="p-3">
              <div className="w-full">
                {/* Header with tabs and expand button */}
                <div className="flex items-center justify-between mb-2">
                  {/* Simple tabs for out/err */}
                  {hasStdout && hasStderr ? (
                    <div className="flex items-center gap-1">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setActiveLogTab('stdout');
                        }}
                        className={cn(
                          'px-3 py-1 text-xs font-medium transition-colors rounded',
                          activeLogTab === 'stdout'
                            ? 'bg-slate-700 dark:bg-slate-600 text-white'
                            : 'bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-300 dark:hover:bg-slate-600'
                        )}
                      >
                        out
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setActiveLogTab('stderr');
                        }}
                        className={cn(
                          'px-3 py-1 text-xs font-medium transition-colors rounded',
                          activeLogTab === 'stderr'
                            ? 'bg-slate-700 dark:bg-slate-600 text-white'
                            : 'bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-300 dark:hover:bg-slate-600'
                        )}
                      >
                        err
                      </button>
                    </div>
                  ) : (
                    <div className="text-xs font-medium text-slate-600 dark:text-slate-400">
                      {hasStdout ? 'stdout' : 'stderr'}
                    </div>
                  )}

                  {/* Expand to modal button */}
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      if (onViewLog) {
                        onViewLog(
                          currentStream === 'stderr'
                            ? `${node.step.name}_stderr`
                            : node.step.name,
                          dagRunId || '',
                          node
                        );
                      }
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
                    title="Open in full modal"
                  >
                    <Code className="h-3 w-3" />
                    <span>Full view</span>
                  </button>
                </div>

                {/* Simple inline log viewer - no controls */}
                <InlineLogViewer
                  dagName={dagRun?.name || name}
                  dagRunId={dagRunId || ''}
                  stepName={node.step.name}
                  stream={currentStream}
                  dagRun={dagRun}
                />
              </div>
            </TableCell>
          </StyledTableRow>
        )}

        {/* Status Update Modal */}
        <StatusUpdateModal
          visible={showStatusModal}
          dismissModal={() => setShowStatusModal(false)}
          step={node.step}
          onSubmit={handleStatusUpdate}
        />
      </>
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
            {hasSubDAGRun && (
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

      {/* Sub dagRun link */}
      {hasSubDAGRun && (
        <>
          {allSubRuns.length === 1 ? (
            // Single sub DAG run
            <>
              <div
                className="text-xs text-blue-500 dark:text-blue-400 font-medium cursor-pointer hover:underline mb-1"
                onClick={(e) => handleSubDAGRunNavigation(0, e)}
              >
                View Sub DAG Run: {subDagName}
              </div>
              {allSubRuns[0]?.params && (
                <div className="text-xs text-slate-500 dark:text-slate-400 mb-3">
                  Parameters:{' '}
                  <span className="font-mono">{allSubRuns[0].params}</span>
                </div>
              )}
            </>
          ) : (
            // Multiple sub DAG runs (parallel execution or repeated)
            <div className="mb-3">
              <SubDAGRunsList
                dagName={dagName}
                dagRunId={dagRunId || ''}
                subDagName={subDagName || ''}
                allSubRuns={allSubRuns}
                isExpanded={isExpanded}
                onToggleExpand={() => setIsExpanded(!isExpanded)}
                onNavigate={handleSubDAGRunNavigation}
              />
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
                        onViewLog(
                          `${node.step.name}_stderr`,
                          dagRunId || '',
                          node
                        );
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
                    onViewLog(`${node.step.name}_stderr`, dagRunId || '', node);
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

      {dagRunId && (
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
                This will re-execute <b>{node.step.name}</b>. Are you sure?
                {error && <div className="text-red-500 mt-2">{error}</div>}
                {success && (
                  <div className="text-green-600 mt-2">Retry started!</div>
                )}
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
      )}
    </div>
  );
}

export default NodeStatusTableRow;
