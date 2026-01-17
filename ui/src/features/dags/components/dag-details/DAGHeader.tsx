import { Calendar, RefreshCw, Server, Terminal, Timer } from 'lucide-react';
import React, { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import { RootDAGRunContext } from '../../contexts/RootDAGRunContext';
import { DAGActions } from '../common';

interface DAGHeaderProps {
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  currentDAGRun: components['schemas']['DAGRunDetails'];
  fileName: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  navigateToStatusTab?: () => void;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  currentDAGRun,
  fileName,
  refreshFn,
  formatDuration,
  navigateToStatusTab,
}) => {
  const navigate = useNavigate();
  const params = useParams<{ tab?: string }>();
  const rootDAGRunContext = React.useContext(RootDAGRunContext);
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const [currentDuration, setCurrentDuration] = React.useState<string>('--');

  // Use the DAG-run from context if available, otherwise use the prop
  const dagRunToDisplay = rootDAGRunContext.data || currentDAGRun;

  // Calculate duration between start and end times
  const calculateDuration = React.useCallback(() => {
    if (!dagRunToDisplay.startedAt || dagRunToDisplay.startedAt === '-') {
      return '--';
    }

    const end =
      dagRunToDisplay.finishedAt && dagRunToDisplay.finishedAt !== '-'
        ? dagRunToDisplay.finishedAt
        : dayjs().toISOString();

    return formatDuration(dagRunToDisplay.startedAt, end);
  }, [dagRunToDisplay.startedAt, dagRunToDisplay.finishedAt, formatDuration]);

  // Determine if the DAG is currently running
  const isRunning = dagRunToDisplay.status === Status.Running;

  // Auto-update duration every second for running DAGs
  useEffect(() => {
    if (isRunning && dagRunToDisplay.startedAt) {
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
  }, [isRunning, dagRunToDisplay.startedAt, dagRunToDisplay.finishedAt]);

  const handleRootDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`
    );
  };

  const handleParentDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?subDAGRunId=${dagRunToDisplay.parentDAGRunId}&dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`
    );
  };

  const handleRefresh = () => {
    setIsRefreshing(true);
    refreshFn();
    setTimeout(() => setIsRefreshing(false), 600);
  };

  // Add keyboard shortcut for refresh
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      // Get current tab (default to 'status' if not set)
      const currentTab = params.tab || 'status';

      // Only trigger on status tab and when not typing
      if (currentTab !== 'status') return;

      // Check if user is typing in an input field
      const target = e.target as HTMLElement;
      if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.contentEditable === 'true' ||
        target.closest('.monaco-editor') ||
        target.closest('[role="textbox"]')
      ) {
        return;
      }

      // Check for 'r' key without modifiers
      if (
        e.key === 'r' &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        !e.shiftKey
      ) {
        e.preventDefault();
        handleRefresh();
      }
    };

    window.addEventListener('keydown', handleKeyPress);
    return () => window.removeEventListener('keydown', handleKeyPress);
  }, [params.tab, handleRefresh]);

  return (
    <div className="bg-card rounded-2xl p-6 mb-6 border border-border shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground mb-2">
            {dagRunToDisplay.rootDAGRunId !== dagRunToDisplay.dagRunId && (
              <>
                <a
                  href={`/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`}
                  onClick={handleRootDAGRunClick}
                  className="text-primary hover:text-primary hover:underline transition-colors font-medium"
                >
                  {dagRunToDisplay.rootDAGRunName}
                </a>
                <span className="text-muted-foreground mx-1">/</span>
              </>
            )}

            {dagRunToDisplay.parentDAGRunName &&
              dagRunToDisplay.parentDAGRunId &&
              dagRunToDisplay.parentDAGRunName !==
                dagRunToDisplay.rootDAGRunName &&
              dagRunToDisplay.parentDAGRunName !== dagRunToDisplay.name && (
                <>
                  <a
                    href={`/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&subDAGRunId=${dagRunToDisplay.parentDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`}
                    onClick={handleParentDAGRunClick}
                    className="text-primary hover:text-primary hover:underline transition-colors font-medium"
                  >
                    {dagRunToDisplay.parentDAGRunName}
                  </a>
                  <span className="text-muted-foreground mx-1">/</span>
                </>
              )}
          </nav>

          <h1 className="text-2xl font-bold text-foreground truncate">
            {dagRunToDisplay.name}
          </h1>
        </div>

        {/* Actions */}
        {dagRunToDisplay.dagRunId === dagRunToDisplay.rootDAGRunId && (
          <div className="flex-shrink-0">
            <DAGActions
              status={dagRunToDisplay}
              dag={dag}
              fileName={fileName}
              refresh={refreshFn}
              displayMode="compact"
              navigateToStatusTab={navigateToStatusTab}
            />
          </div>
        )}
      </div>

      {/* Status and metadata row */}
      {dagRunToDisplay.status !== undefined &&
        dagRunToDisplay.status !== null && (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <StatusChip status={dagRunToDisplay.status} size="md">
              {dagRunToDisplay.statusLabel || ''}
            </StatusChip>

            <button
              onClick={handleRefresh}
              disabled={isRefreshing}
              className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              title="Refresh (R)"
            >
              <RefreshCw
                className={`h-3 w-3 ${isRefreshing ? 'animate-spin' : ''}`}
              />
              <span>Refresh</span>
            </button>

            <div className="flex items-center gap-1.5 text-foreground bg-accent rounded-md px-2 py-1 border">
              <Calendar className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium text-xs">
                {dagRunToDisplay?.startedAt
                  ? `${dayjs(dagRunToDisplay.startedAt).format('MMM D, HH:mm:ss')} ${dayjs(dagRunToDisplay.startedAt).format('z')}`
                  : '--'}
              </span>
            </div>

            <div className="flex items-center gap-1.5 text-foreground bg-accent rounded-md px-2 py-1 border">
              <Timer className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium text-xs flex items-center gap-1">
                {currentDuration}
                {isRunning && (
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
                )}
              </span>
            </div>

            {dagRunToDisplay.workerId && (
              <div className="flex items-center gap-1.5 text-foreground bg-accent rounded-md px-2 py-1 border">
                <Server className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="font-medium text-xs font-mono">
                  {dagRunToDisplay.workerId}
                </span>
              </div>
            )}

            <div className="flex items-center gap-1.5 text-muted-foreground">
              <span className="font-medium text-xs uppercase tracking-wide">
                Run ID
              </span>
              <code className="bg-accent text-foreground px-2 py-1 rounded-md text-xs font-mono border">
                {dagRunToDisplay.rootDAGRunId}
              </code>
            </div>
          </div>
        )}

      {/* Parameters - Always show to prevent layout jumping */}
      <div className="mt-4 border-t border-border pt-4">
        <div className="flex items-center gap-2 mb-2">
          <Terminal className="h-4 w-4 text-muted-foreground" />
          <span className="text-xs font-semibold text-foreground/90">
            Parameters
          </span>
        </div>
        <div className="flex items-center bg-accent rounded-md px-3 py-1.5 font-mono text-xs min-h-[36px] max-h-[100px] overflow-y-auto border">
          {dagRunToDisplay.params ? (
            <span className="text-foreground">{dagRunToDisplay.params}</span>
          ) : (
            <span className="text-muted-foreground italic">No parameters</span>
          )}
        </div>
      </div>
    </div>
  );
};

export default DAGHeader;
