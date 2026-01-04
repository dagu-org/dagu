/**
 * NodeStatusTableRow component renders a single row in the node status table.
 *
 * @module features/dags/components/dag-details
 */
import { Button } from '@/components/ui/button';
import { CommandDisplay } from '@/components/ui/command-display';
import { ScriptBadge } from '@/components/ui/script-dialog';
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import {
  AlertCircle,
  Ban,
  Check,
  ChevronDown,
  ChevronRight,
  Code,
  GitBranch,
  Play,
  RefreshCw,
  X,
} from 'lucide-react';
import { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  components,
  NodeStatus,
  Status,
  Stream,
} from '../../../../api/v2/schema';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { NodeStatusChip } from '../common';
import { ApprovalModal } from '../dag-execution/ApprovalModal';
import { RejectionModal } from '../dag-execution/RejectionModal';
import StatusUpdateModal from '../dag-execution/StatusUpdateModal';
import { SubDAGRunsList } from './SubDAGRunsList';

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
    <div className="bg-slate-800 rounded overflow-hidden">
      {isLoading && !data ? (
        <div className="text-slate-400 text-xs py-4 px-3">Loading logs...</div>
      ) : lines.length === 0 ? (
        <div className="text-slate-400 text-xs py-4 px-3">
          &lt;No log output&gt;
        </div>
      ) : (
        <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
          <pre className="font-mono text-[11px] text-slate-100 p-2">
            {lines.map((line, index) => {
              const lineNumber = totalLines - lineCount + index + 1;
              return (
                <div key={index} className="flex px-1 py-0.5">
                  <span className="text-slate-500 mr-3 select-none w-12 text-right flex-shrink-0">
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
  // State for approval modal
  const [showApprovalModal, setShowApprovalModal] = useState(false);
  // State for rejection modal
  const [showRejectionModal, setShowRejectionModal] = useState(false);
  // Check if this is a sub dagRun node
  // Include both regular and repeated sub runs
  const allSubRuns = [...(node.subRuns || []), ...(node.subRunsRepeated || [])];
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
        return 'bg-success-muted';
      case NodeStatus.Failed:
        return 'bg-error-muted';
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

  // Handle approval for wait steps
  const handleApprove = async (inputs: Record<string, string>) => {
    // Check if this is a sub DAG-run
    const isSubDAGRun =
      dagRun.rootDAGRunId &&
      dagRun.rootDAGRunName &&
      dagRun.rootDAGRunId !== dagRun.dagRunId;

    // Define path parameters
    const pathParams = {
      name: isSubDAGRun ? dagRun.rootDAGRunName : dagName,
      dagRunId: isSubDAGRun ? dagRun.rootDAGRunId : dagRunId || '',
      stepName: node.step.name,
      ...(isSubDAGRun ? { subDAGRunId: dagRun.dagRunId } : {}),
    };

    // Use the appropriate endpoint
    const endpoint = isSubDAGRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/approve'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/approve';

    const { error } = await client.POST(endpoint, {
      params: {
        path: pathParams,
        query: {
          remoteNode,
        },
      },
      body: {
        inputs: Object.keys(inputs).length > 0 ? inputs : undefined,
      },
    });

    if (error) {
      throw new Error(error.message || 'Failed to approve step');
    }
  };

  // Handle rejection for wait steps
  const handleReject = async (reason: string) => {
    // Check if this is a sub DAG-run
    const isSubDAGRun =
      dagRun.rootDAGRunId &&
      dagRun.rootDAGRunName &&
      dagRun.rootDAGRunId !== dagRun.dagRunId;

    // Define path parameters
    const pathParams = {
      name: isSubDAGRun ? dagRun.rootDAGRunName : dagName,
      dagRunId: isSubDAGRun ? dagRun.rootDAGRunId : dagRunId || '',
      stepName: node.step.name,
      ...(isSubDAGRun ? { subDAGRunId: dagRun.dagRunId } : {}),
    };

    // Use the appropriate endpoint
    const endpoint = isSubDAGRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/reject'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/reject';

    const { error } = await client.POST(endpoint, {
      params: {
        path: pathParams,
        query: {
          remoteNode,
        },
      },
      body: {
        reason: reason || undefined,
      },
    });

    if (error) {
      throw new Error(error.message || 'Failed to reject step');
    }
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
            'hover:bg-muted transition-colors duration-200 h-auto cursor-pointer',
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
                  className="text-muted-foreground hover:text-foreground/90"
                >
                  {isLogExpanded ? (
                    <ChevronDown className="h-4 w-4" />
                  ) : (
                    <ChevronRight className="h-4 w-4" />
                  )}
                </button>
              )}
              <span className="font-semibold text-foreground/90 text-xs">
                {rownum}
              </span>
            </div>
          </TableCell>

          {/* Combined Step Name & Description */}
          <TableCell>
            <div className="space-y-0.5">
              <div className="text-sm font-semibold text-foreground text-wrap break-all flex items-center gap-1.5">
                {node.step.name}
                {hasSubDAGRun && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="inline-flex items-center text-primary cursor-pointer">
                        <GitBranch className="h-4 w-4" />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span className="text-xs">Sub DAG Run: {subDagName}</span>
                    </TooltipContent>
                  </Tooltip>
                )}
              </div>
              {node.step.description && (
                <div className="text-xs text-muted-foreground leading-tight">
                  {node.step.description}
                </div>
              )}

              {/* Repeat Policy */}
              {node.step.repeatPolicy?.repeat && (
                <div className="flex items-start gap-1 mt-1">
                  <span
                    className={`inline-flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider px-1.5 py-0.5 rounded ${
                      node.step.repeatPolicy.repeat === 'while'
                        ? 'bg-info-muted text-info'
                        : node.step.repeatPolicy.repeat === 'until'
                          ? 'bg-info-muted text-info'
                          : 'bg-primary/15 text-primary'
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
                    <span className="text-[10px] text-muted-foreground font-mono">
                      {node.step.repeatPolicy.condition.condition}
                      {node.step.repeatPolicy.condition.expected && (
                        <span className="text-success">
                          ={node.step.repeatPolicy.condition.expected}
                        </span>
                      )}
                    </span>
                  )}

                  {node.step.repeatPolicy.exitCode &&
                    node.step.repeatPolicy.exitCode.length > 0 && (
                      <span className="text-[10px] text-muted-foreground font-mono">
                        exit:[{node.step.repeatPolicy.exitCode.join(',')}]
                      </span>
                    )}
                </div>
              )}

              {hasSubDAGRun && (
                <SubDAGRunsList
                  dagName={dagRun.name}
                  dagRunId={dagRun.dagRunId}
                  rootDagName={dagRun.rootDAGRunName}
                  rootDagRunId={dagRun.rootDAGRunId || ''}
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
            <div className="space-y-1.5">
              {node.step.commands && node.step.commands.length > 0 && (
                <CommandDisplay
                  commands={node.step.commands}
                  icon="code"
                  maxLength={50}
                />
              )}
              {node.step.script && (
                <ScriptBadge
                  script={node.step.script}
                  stepName={node.step.name}
                />
              )}
            </div>
          </TableCell>

          {/* Last Run & Duration */}
          <TableCell>
            <div className="space-y-0.5">
              <div className="font-medium text-foreground/90 text-sm">
                {formatTimestamp(node.startedAt)}
              </div>
              {node.startedAt && (
                <div className="text-xs text-muted-foreground flex items-center gap-1.5 leading-tight">
                  <span className="font-medium flex items-center">
                    Duration:
                    {node.status === NodeStatus.Running && (
                      <span className="inline-block w-2 h-2 rounded-full bg-success ml-1.5 animate-pulse" />
                    )}
                  </span>
                  {currentDuration}
                </div>
              )}
              {/* Approval info for HITL steps */}
              {node.approvedBy && (
                <div className="text-xs text-muted-foreground leading-tight">
                  <span className="font-medium">Approved by:</span>{' '}
                  <span className="text-info">{node.approvedBy}</span>
                  {node.approvedAt && (
                    <span className="ml-1">
                      at {formatTimestamp(node.approvedAt)}
                    </span>
                  )}
                </div>
              )}
              {node.approvalInputs && Object.keys(node.approvalInputs).length > 0 && (
                <div className="text-xs text-muted-foreground leading-tight">
                  <span className="font-medium">Inputs:</span>{' '}
                  <span className="font-mono text-foreground/80">
                    {JSON.stringify(node.approvalInputs)}
                  </span>
                </div>
              )}
              {/* Rejection info for rejected HITL steps */}
              {node.rejectedBy && (
                <div className="text-xs text-muted-foreground leading-tight">
                  <span className="font-medium">Rejected by:</span>{' '}
                  <span className="text-error">{node.rejectedBy}</span>
                  {node.rejectedAt && (
                    <span className="ml-1">
                      at {formatTimestamp(node.rejectedAt)}
                    </span>
                  )}
                </div>
              )}
              {node.rejectionReason && (
                <div className="text-xs text-muted-foreground leading-tight">
                  <span className="font-medium">Reason:</span>{' '}
                  <span className="text-foreground/80">{node.rejectionReason}</span>
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
                <div className="inline-flex items-center rounded-md overflow-hidden border border-border shadow-sm">
                  {/* stdout button */}
                  {node.stdout && (
                    <>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <a
                            href={url}
                            onClick={(e) => {
                              e.stopPropagation();
                              handleViewLog(e);
                            }}
                            className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium transition-colors bg-card hover:bg-muted cursor-pointer"
                            title="View stdout log (Cmd/Ctrl+Click for new tab)"
                          >
                            <Code className="h-3.5 w-3.5 text-muted-foreground" />
                            <span>stdout</span>
                          </a>
                        </TooltipTrigger>
                        <TooltipContent>
                          <span className="text-xs">Standard Output Log</span>
                        </TooltipContent>
                      </Tooltip>
                      {node.stderr && <div className="w-px h-5 bg-border" />}
                    </>
                  )}

                  {/* stderr button */}
                  {node.stderr && (
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
                          className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium transition-colors bg-card hover:bg-warning/10 cursor-pointer"
                          title="View stderr log (Cmd/Ctrl+Click for new tab)"
                        >
                          <AlertCircle className="h-3.5 w-3.5 text-warning" />
                          <span className="text-warning">stderr</span>
                        </a>
                      </TooltipTrigger>
                      <TooltipContent>
                        <span className="text-xs">Error Output Log</span>
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
              )}

              {/* Errors - Simplified */}
              {node.error && (
                <div className="text-xs text-error leading-relaxed whitespace-normal break-words">
                  {node.error}
                </div>
              )}
              {node.step.preconditions?.some((cond) => cond.error) && (
                <div className="text-xs text-warning leading-relaxed">
                  Precondition unmet
                </div>
              )}
            </div>
          </TableCell>
          {dagRunId && (
            <TableCell className="text-center">
              <div className="flex items-center justify-center gap-1">
                {/* Approve button for waiting steps */}
                {node.status === NodeStatus.Waiting && (
                  <>
                    <Button
                      size="icon-sm"
                      className="btn-3d-secondary bg-warning/10 hover:bg-warning/20"
                      title="Approve this step"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowApprovalModal(true);
                      }}
                    >
                      <Check className="h-4 w-4 text-warning" />
                    </Button>
                    <Button
                      size="icon-sm"
                      className="btn-3d-secondary bg-error/10 hover:bg-error/20"
                      title="Reject this step"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowRejectionModal(true);
                      }}
                    >
                      <Ban className="h-4 w-4 text-error" />
                    </Button>
                  </>
                )}
                {/* Retry button - hidden for Waiting and Rejected steps */}
                {node.status !== NodeStatus.Waiting && node.status !== NodeStatus.Rejected && (
                  <Button
                    size="icon-sm"
                    className="btn-3d-secondary"
                    title="Retry from this step"
                    onClick={(e) => {
                      e.stopPropagation();
                      setShowDialog(true);
                    }}
                    disabled={loading || dagRun.status === Status.Running}
                  >
                    <Play className="h-4 w-4 text-success" />
                  </Button>
                )}
              </div>
              <Dialog open={showDialog} onOpenChange={setShowDialog}>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Retry from this step?</DialogTitle>
                  </DialogHeader>
                  <div className="py-2 text-sm">
                    This will re-execute <b>{node.step.name}</b>. Are you sure?
                    {error && <div className="text-error mt-2">{error}</div>}
                    {success && (
                      <div className="text-success mt-2">Retry started!</div>
                    )}
                  </div>
                  <DialogFooter>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowDialog(false)}
                      disabled={loading}
                    >
                      <X className="h-4 w-4" />
                      Cancel
                    </Button>
                    <Button
                      size="sm"
                      onClick={handleRetry}
                      disabled={loading}
                    >
                      <Play className="h-4 w-4" />
                      {loading ? 'Retrying...' : 'Retry'}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </TableCell>
          )}
        </StyledTableRow>

        {/* Inline log viewer row - spans entire table width */}
        {isLogExpanded && hasLogs && (
          <StyledTableRow className="bg-muted">
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
                            ? 'bg-foreground/80 text-white'
                            : 'bg-accent text-foreground/90 hover:bg-accent'
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
                            ? 'bg-foreground/80 text-white'
                            : 'bg-accent text-foreground/90 hover:bg-accent'
                        )}
                      >
                        err
                      </button>
                    </div>
                  ) : (
                    <div className="text-xs font-medium text-muted-foreground">
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
                    className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-primary hover:text-primary transition-colors"
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

        {/* Approval Modal for wait steps */}
        <ApprovalModal
          visible={showApprovalModal}
          dismissModal={() => setShowApprovalModal(false)}
          step={node.step}
          onApprove={handleApprove}
        />

        {/* Rejection Modal for wait steps */}
        <RejectionModal
          visible={showRejectionModal}
          dismissModal={() => setShowRejectionModal(false)}
          step={node.step}
          onReject={handleReject}
        />
      </>
    );
  }

  // Render mobile view (card)
  return (
    <div
      className={cn(
        'p-4 rounded-2xl border border-border bg-card hover:',
        getRowHighlight()
      )}
    >
      {/* Header with number and status */}
      <div className="flex justify-between items-center mb-3">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-foreground/90 text-sm bg-muted rounded-full w-6 h-6 flex items-center justify-center">
            {rownum}
          </span>
          <h3 className="font-semibold text-foreground">
            {node.step.name}
            {hasSubDAGRun && (
              <span className="inline-flex items-center text-primary ml-1.5">
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
        <div className="text-xs text-muted-foreground mb-3">
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
                className="text-xs text-primary font-medium cursor-pointer hover:underline mb-1"
                onClick={(e) => handleSubDAGRunNavigation(0, e)}
              >
                View Sub DAG Run: {subDagName}
              </div>
              {allSubRuns[0]?.params && (
                <div className="text-xs text-muted-foreground mb-3">
                  Parameters:{' '}
                  <span className="font-mono">{allSubRuns[0].params}</span>
                </div>
              )}
            </>
          ) : (
            // Multiple sub DAG runs (parallel execution or repeated)
            <div className="mb-3">
              <SubDAGRunsList
                dagName={dagRun.name}
                dagRunId={dagRun.dagRunId}
                rootDagName={dagRun.rootDAGRunName}
                rootDagRunId={dagRun.rootDAGRunId || ''}
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
        <div className="text-xs font-medium text-foreground/90 mb-1">
          {node.step.commands && node.step.commands.length > 1 ? 'Commands:' : 'Command:'}
        </div>
        <div className="space-y-1.5">
          {node.step.commands && node.step.commands.length > 0 && (
            <div className="space-y-1.5">
              {node.step.commands.map((entry, idx) => {
                const fullCmd = entry.args && entry.args.length > 0
                  ? `${entry.command} ${entry.args.join(' ')}`
                  : entry.command;
                return (
                  <div key={idx} className="flex items-start gap-1.5 text-xs">
                    {node.step.commands && node.step.commands.length > 1 ? (
                      <span className="text-muted-foreground font-mono w-4 text-right flex-shrink-0 mt-1">
                        {idx + 1}.
                      </span>
                    ) : (
                      <Code className="h-4 w-4 text-primary flex-shrink-0 mt-0.5" />
                    )}
                    <code className="bg-muted rounded-md px-2 py-1 text-foreground/90 break-all whitespace-pre-wrap font-mono text-xs flex-1">
                      {fullCmd}
                    </code>
                  </div>
                );
              })}
            </div>
          )}

          {node.step.script && (
            <ScriptBadge script={node.step.script} stepName={node.step.name} />
          )}
        </div>
      </div>

      {/* Timing section */}
      <div className="mb-3">
        <div className="text-xs font-medium text-foreground/90 mb-1">
          Timing:
        </div>
        <div className="space-y-0.5">
          <div className="text-xs text-muted-foreground">
            Started: {formatTimestamp(node.startedAt)}
          </div>
          {node.startedAt && (
            <div className="text-xs text-muted-foreground flex items-center gap-1.5">
              <span className="font-medium flex items-center">
                Duration:
                {node.status === NodeStatus.Running && (
                  <span className="inline-block w-2 h-2 rounded-full bg-success ml-1.5 animate-pulse" />
                )}
              </span>
              {currentDuration}
            </div>
          )}
          {/* Approval info for HITL steps */}
          {node.approvedBy && (
            <div className="text-xs text-muted-foreground">
              <span className="font-medium">Approved by:</span>{' '}
              <span className="text-info">{node.approvedBy}</span>
              {node.approvedAt && (
                <span className="ml-1">
                  at {formatTimestamp(node.approvedAt)}
                </span>
              )}
            </div>
          )}
          {node.approvalInputs && Object.keys(node.approvalInputs).length > 0 && (
            <div className="text-xs text-muted-foreground">
              <span className="font-medium">Inputs:</span>{' '}
              <span className="font-mono text-foreground/80">
                {JSON.stringify(node.approvalInputs)}
              </span>
            </div>
          )}
          {/* Rejection info for rejected HITL steps */}
          {node.rejectedBy && (
            <div className="text-xs text-muted-foreground">
              <span className="font-medium">Rejected by:</span>{' '}
              <span className="text-error">{node.rejectedBy}</span>
              {node.rejectedAt && (
                <span className="ml-1">
                  at {formatTimestamp(node.rejectedAt)}
                </span>
              )}
            </div>
          )}
          {node.rejectionReason && (
            <div className="text-xs text-muted-foreground">
              <span className="font-medium">Reason:</span>{' '}
              <span className="text-foreground/80">{node.rejectionReason}</span>
            </div>
          )}
        </div>
      </div>

      {/* Error section */}
      {(node.error || node.step.preconditions?.some((cond) => cond.error)) && (
        <div className="mb-3">
          <div className="text-xs font-medium text-foreground/90 mb-1">
            Errors:
          </div>

          {node.error && (
            <div className="text-xs bg-error-muted border border-error/20 rounded-md p-1.5 mb-2 whitespace-pre-wrap break-words text-error leading-tight">
              {node.error}
            </div>
          )}

          {node.step.preconditions?.some((cond) => cond.error) && (
            <div>
              <div className="text-xs font-medium text-warning mb-1">
                Precondition Unmet:
              </div>
              {node.step.preconditions
                .filter((cond) => cond.error)
                .map((cond, idx) => (
                  <div
                    key={idx}
                    className="text-xs bg-warning-muted border border-warning/20 rounded-md p-1.5 mb-1 whitespace-pre-wrap break-words text-warning leading-tight"
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
          <div className="inline-flex items-center rounded-md overflow-hidden border border-border shadow-sm">
            {/* stdout button */}
            {node.stdout && (
              <>
                <a
                  href={url}
                  onClick={handleViewLog}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors bg-card hover:bg-muted cursor-pointer"
                  title="View stdout log (Cmd/Ctrl+Click for new tab)"
                >
                  <Code className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>stdout</span>
                </a>
                {node.stderr && <div className="w-px h-5 bg-border" />}
              </>
            )}

            {/* stderr button */}
            {node.stderr && (
              <a
                href={`${url}&stream=stderr`}
                onClick={(e) => {
                  if (!(e.metaKey || e.ctrlKey) && onViewLog) {
                    e.preventDefault();
                    onViewLog(`${node.step.name}_stderr`, dagRunId || '', node);
                  }
                }}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors bg-card hover:bg-warning/10 cursor-pointer"
                title="View stderr log (Cmd/Ctrl+Click for new tab)"
              >
                <AlertCircle className="h-3.5 w-3.5 text-warning" />
                <span className="text-warning">stderr</span>
              </a>
            )}
          </div>
        </div>
      )}

      {dagRunId && (
        <div className="flex justify-end mt-4 gap-2">
          {/* Approve and Reject buttons for waiting steps */}
          {node.status === NodeStatus.Waiting && (
            <>
              <button
                className="p-2 rounded-full hover:bg-warning/20 bg-warning/10"
                title="Approve this step"
                onClick={() => setShowApprovalModal(true)}
              >
                <Check className="h-6 w-6 text-warning" />
              </button>
              <button
                className="p-2 rounded-full hover:bg-error/20 bg-error/10"
                title="Reject this step"
                onClick={() => setShowRejectionModal(true)}
              >
                <Ban className="h-6 w-6 text-error" />
              </button>
            </>
          )}
          {/* Retry button - hidden for Waiting and Rejected steps */}
          {node.status !== NodeStatus.Waiting && node.status !== NodeStatus.Rejected && (
            <button
              className="p-2 rounded-full hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed"
              title="Retry from this step"
              onClick={() => setShowDialog(true)}
              disabled={loading || dagRun.status === Status.Running}
            >
              <Play className="h-6 w-6 text-success" />
            </button>
          )}
          <Dialog open={showDialog} onOpenChange={setShowDialog}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Retry from this step?</DialogTitle>
              </DialogHeader>
              <div className="py-2 text-sm">
                This will re-execute <b>{node.step.name}</b>. Are you sure?
                {error && <div className="text-error mt-2">{error}</div>}
                {success && (
                  <div className="text-success mt-2">Retry started!</div>
                )}
              </div>
              <DialogFooter>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setShowDialog(false)}
                  disabled={loading}
                >
                  <X className="h-4 w-4" />
                  Cancel
                </Button>
                <Button
                  size="sm"
                  onClick={handleRetry}
                  disabled={loading}
                >
                  <Play className="h-4 w-4" />
                  {loading ? 'Retrying...' : 'Retry'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Approval Modal for wait steps */}
          <ApprovalModal
            visible={showApprovalModal}
            dismissModal={() => setShowApprovalModal(false)}
            step={node.step}
            onApprove={handleApprove}
          />

          {/* Rejection Modal for wait steps */}
          <RejectionModal
            visible={showRejectionModal}
            dismissModal={() => setShowRejectionModal(false)}
            step={node.step}
            onReject={handleReject}
          />
        </div>
      )}
    </div>
  );
}

export default NodeStatusTableRow;
