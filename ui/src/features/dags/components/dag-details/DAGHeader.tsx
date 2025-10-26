import { Calendar, Terminal, Timer, RefreshCw } from 'lucide-react';
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
        ? dayjs(dagRunToDisplay.finishedAt)
        : dayjs();

    const start = dayjs(dagRunToDisplay.startedAt);
    const diff = end.diff(start, 'second');

    const hours = Math.floor(diff / 3600);
    const minutes = Math.floor((diff % 3600) / 60);
    const seconds = diff % 60;

    return `${hours > 0 ? `${hours}h ` : ''}${minutes > 0 ? `${minutes}m ` : ''}${seconds}s`;
  }, [dagRunToDisplay.startedAt, dagRunToDisplay.finishedAt]);

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
      if (e.key === 'r' && !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
        e.preventDefault();
        handleRefresh();
      }
    };

    window.addEventListener('keydown', handleKeyPress);
    return () => window.removeEventListener('keydown', handleKeyPress);
  }, [params.tab, handleRefresh]);

  return (
    <div className="bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 rounded-2xl p-6 mb-6 border border-slate-200 dark:border-slate-700 shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-slate-600 dark:text-slate-400 mb-2">
            {dagRunToDisplay.rootDAGRunId !== dagRunToDisplay.dagRunId && (
              <>
                <a
                  href={`/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`}
                  onClick={handleRootDAGRunClick}
                  className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                >
                  {dagRunToDisplay.rootDAGRunName}
                </a>
                <span className="text-slate-400 dark:text-slate-500 mx-1">
                  /
                </span>
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
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                  >
                    {dagRunToDisplay.parentDAGRunName}
                  </a>
                  <span className="text-slate-400 dark:text-slate-500 mx-1">
                    /
                  </span>
                </>
              )}
          </nav>

          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 truncate">
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
          <div className="flex flex-wrap items-center gap-4 text-sm">
            {/* Status and Refresh */}
            <div className="flex items-center gap-2">
              {dagRunToDisplay.status !== undefined && (
                <StatusChip status={dagRunToDisplay.status} size="md">
                  {dagRunToDisplay.statusLabel || ''}
                </StatusChip>
              )}
              <button
                onClick={handleRefresh}
                disabled={isRefreshing}
                className="relative group inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-md text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100 hover:bg-slate-100 dark:hover:bg-slate-700/50 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                title="Refresh (R)"
              >
                <RefreshCw className={`h-3 w-3 ${isRefreshing ? 'animate-spin' : ''}`} />
                <span>Refresh</span>
                <span className="absolute -bottom-1 -right-1 bg-primary text-primary-foreground text-[10px] font-medium px-1 rounded-sm opacity-0 group-hover:opacity-100 transition-opacity">
                  R
                </span>
              </button>
            </div>

            {/* Metadata items */}
            <div className="flex flex-wrap items-center gap-4 lg:gap-6 text-sm">
              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200 bg-slate-200 dark:bg-slate-700 rounded-md px-3 py-1.5 border">
                <Calendar className="h-4 w-4 text-slate-600 dark:text-slate-400" />
                <span className="font-medium text-xs">
                  {dagRunToDisplay?.startedAt
                    ? `${dayjs(dagRunToDisplay.startedAt).format('MMM D, HH:mm:ss')} ${dayjs(dagRunToDisplay.startedAt).format('z')}`
                    : '--'}
                </span>
              </div>

              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200 bg-slate-200 dark:bg-slate-700 rounded-md px-3 py-1.5 border">
                <Timer className="h-4 w-4 text-slate-600 dark:text-slate-400" />
                <span className="font-medium text-xs flex items-center gap-1">
                  {currentDuration}
                  {isRunning && (
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-lime-500 animate-pulse" />
                  )}
                </span>
              </div>

              <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 ml-auto">
                <span className="font-medium text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                  Run ID
                </span>
                <code className="bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 px-3 py-1.5 rounded-md text-xs font-mono border">
                  {dagRunToDisplay.rootDAGRunId}
                </code>
              </div>
            </div>
          </div>
        )}

      {/* Parameters - Show if present */}
      {dagRunToDisplay.params && (
        <div className="mt-4 border-t border-slate-200 dark:border-slate-700 pt-4">
          <div className="flex items-center gap-2 mb-2">
            <Terminal className="h-4 w-4 text-slate-500" />
            <span className="text-sm font-semibold text-slate-700 dark:text-slate-300">
              Parameters
            </span>
          </div>
          <div className="bg-slate-200 dark:bg-slate-700 rounded-md px-4 py-3 font-mono text-sm text-slate-800 dark:text-slate-200 max-h-[120px] overflow-y-auto border">
            {dagRunToDisplay.params}
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGHeader;
