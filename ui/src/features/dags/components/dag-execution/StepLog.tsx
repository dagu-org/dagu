/**
 * StepLog component displays the execution log for a specific step in a DAG run.
 *
 * @module features/dags/components/dag-execution
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import { ReloadButton } from '../../../../components/ui/reload-button';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';

// Extended Log type with pagination fields
interface LogWithPagination {
  content: string;
  lineCount?: number;
  totalLines?: number;
  hasMore?: boolean;
  isEstimate?: boolean;
}

// Calculate total pages based on total lines and page size
const calculateTotalPages = (totalLines: number, pageSize: number): number => {
  return Math.ceil(totalLines / pageSize);
};

/**
 * Props for the StepLog component
 */
type Props = {
  /** DAG name or fileName */
  dagName: string;
  /** DAG-run ID of the execution */
  dagRunId: string;
  /** Name of the step to display logs for */
  stepName: string;
  /** Full DAG-run details (optional) - used to determine if this is a sub DAG-run */
  dagRun?: components['schemas']['DAGRunDetails'];
  /** Whether to show stdout or stderr logs */
  stream?: 'stdout' | 'stderr';
  /** Node information (optional) - contains repeated log files */
  node?: components['schemas']['Node'];
};

/**
 * Regular expression to match ANSI color codes for removal
 * Credit: https://github.com/chalk/ansi-regex/commit/02fa893d619d3da85411acc8fd4e2eea0e95a9d9 under MIT license
 */
const ANSI_CODES_REGEX = [
  '[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)',
  '(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))',
].join('|');

/**
 * StepLog displays the log output for a specific step in a DAG run
 * Fetches log data from the API and refreshes every 30 seconds
 */
function StepLog({
  dagName,
  dagRunId,
  stepName,
  dagRun,
  stream = 'stdout',
  node,
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [viewMode, setViewMode] = useState<'tail' | 'head' | 'page'>('tail');
  const [pageSize, setPageSize] = useState(1000);
  const [currentPage, setCurrentPage] = useState(1);
  const [jumpToLine, setJumpToLine] = useState<number | ''>('');
  // Check if the node is running
  const isRunning = node?.status === NodeStatus.Running;
  
  const [isLiveMode, setIsLiveMode] = useState(isRunning);

  // Keep track of previous data to prevent flashing
  const [cachedData, setCachedData] = useState<LogWithPagination | null>(null);
  const [isNavigating, setIsNavigating] = useState(false);

  // Refs to track component state
  const isInitialLoad = useRef(true);
  const navigationTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const logContainerRef = useRef<HTMLDivElement>(null);

  // Determine if this is a sub DAG-run
  const isSubDAGRun =
    dagRun &&
    dagRun.rootDAGRunId &&
    dagRun.rootDAGRunId !== dagRun.dagRunId;

  // Determine query parameters based on view mode
  const queryParams: Record<string, number | string> = {
    remoteNode: appBarContext.selectedRemoteNode || 'local',
    stream: stream,
  };

  // Add pagination parameters based on view mode
  if (viewMode === 'tail') {
    queryParams.tail = pageSize;
  } else if (viewMode === 'head') {
    queryParams.head = pageSize;
  } else if (viewMode === 'page') {
    queryParams.offset = (currentPage - 1) * pageSize + 1;
    queryParams.limit = pageSize;
  }

  // Determine the API endpoint based on whether this is a sub DAG-run
  const apiEndpoint = isSubDAGRun
    ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/log'
    : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/log';

  // Prepare path parameters based on whether this is a sub DAG-run
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

  // Fetch log data with periodic refresh
  const { data, isLoading, error, mutate } = useQuery(
    apiEndpoint,
    {
      params: {
        query: queryParams,
        path: pathParams,
      },
    },
    {
      refreshInterval: isLiveMode && isRunning ? 2000 : 0, // 2s in live mode, 0 (disabled) otherwise
      keepPreviousData: true, // Keep previous data while loading new data
      revalidateOnFocus: false, // Don't revalidate when window regains focus
      dedupingInterval: 1000, // Deduplicate requests within 1 second
    }
  );

  // Function to scroll to the bottom of the log container
  const scrollToBottom = useCallback(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, []);

  // Combined effect to handle data loading and navigation state
  useEffect(() => {
    // When data is received, update cached data and reset navigation state
    if (data) {
      setCachedData(data as LogWithPagination);
      setIsNavigating(false);
      isInitialLoad.current = false;

      // Clear any pending navigation timeout
      if (navigationTimeoutRef.current) {
        clearTimeout(navigationTimeoutRef.current);
        navigationTimeoutRef.current = null;
      }

      // Auto-scroll to bottom when in tail view mode
      if (viewMode === 'tail') {
        // Use setTimeout to ensure the DOM has updated
        setTimeout(scrollToBottom, 100);
      }
    }

    // When loading completes with no data but we have cached data
    if (!isLoading && !data && cachedData && !isInitialLoad.current) {
      setIsNavigating(false);
    }
  }, [data, isLoading, cachedData, viewMode, scrollToBottom]);

  // Set navigating state when changing view parameters
  useEffect(() => {
    // Only set navigating state after initial load
    if (!isInitialLoad.current) {
      setIsNavigating(true);

      // Clear any existing timeout
      if (navigationTimeoutRef.current) {
        clearTimeout(navigationTimeoutRef.current);
      }

      // Set a new safety timeout
      navigationTimeoutRef.current = setTimeout(() => {
        setIsNavigating(false);
        navigationTimeoutRef.current = null;
      }, 3000); // Shorter timeout for better UX
    }

    // Cleanup on unmount
    return () => {
      if (navigationTimeoutRef.current) {
        clearTimeout(navigationTimeoutRef.current);
        navigationTimeoutRef.current = null;
      }
    };
  }, [viewMode, currentPage, pageSize]);

  // Handle navigation actions
  const handleViewModeChange = (mode: 'tail' | 'head' | 'page') => {
    // If already in this mode, don't trigger a reload
    if (mode === viewMode) return;

    setViewMode(mode);
    setCurrentPage(1);

    // If switching to tail view, scroll to bottom after data loads
    // The scrolling will happen in the data effect
  };

  const handlePageChange = (newPage: number) => {
    setCurrentPage(newPage);
  };

  const handleJumpToLine = () => {
    if (
      jumpToLine !== '' &&
      jumpToLine > 0 &&
      jumpToLine <= (cachedData?.totalLines || 0)
    ) {
      // Calculate the target page
      const lineNum = jumpToLine as number;
      const targetPage = Math.ceil(lineNum / pageSize);

      // Set the page and view mode
      setCurrentPage(targetPage);
      setViewMode('page');

      // After the data loads and the component re-renders, scroll to the specific line
      // We need to use setTimeout to ensure the DOM has updated
      setTimeout(() => {
        // Find the line element by its line number
        const lineElements = document.querySelectorAll('[data-line-number]');
        for (let i = 0; i < lineElements.length; i++) {
          const element = lineElements[i] as HTMLElement;
          const lineNumber = parseInt(
            element.getAttribute('data-line-number') || '0',
            10
          );
          if (lineNumber === lineNum) {
            // Scroll the element into view
            element.scrollIntoView({ behavior: 'smooth', block: 'center' });
            // Highlight the line temporarily
            element.classList.add('bg-blue-500', 'bg-opacity-20');
            setTimeout(() => {
              element.classList.remove('bg-blue-500', 'bg-opacity-20');
            }, 2000);
            break;
          }
        }
      }, 500);
    }
  };

  // Show loading indicator only on initial load
  if (isLoading && !cachedData && isInitialLoad.current) {
    return <LoadingIndicator />;
  }

  // Use cached data if available, otherwise use current data
  const logData = (data || cachedData) as LogWithPagination;

  // Handle error state
  if (error && !logData) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <div className="text-red-500">
          Error loading log data: {error.message || 'Unknown error'}
        </div>
      </div>
    );
  }

  // Process log data
  const content =
    logData?.content.replace(new RegExp(ANSI_CODES_REGEX, 'g'), '') || '';
  const lineCount = logData?.lineCount || 0;
  const totalLines = logData?.totalLines || 0;
  const hasMore = logData?.hasMore || false;
  const isEstimate = logData?.isEstimate || false;

  // Split content into lines for better rendering
  const lines = content ? content.split('\n') : ['<No log output>'];

  // Calculate total pages
  const totalPages = calculateTotalPages(totalLines, pageSize);

  // Calculate line numbers based on current page and view mode
  const getLineNumber = (index: number): number => {
    if (viewMode === 'tail') {
      // For tail view, line numbers start from (totalLines - lineCount + 1)
      return totalLines - lineCount + index + 1;
    } else if (viewMode === 'head') {
      // For head view, line numbers start from 1
      return index + 1;
    } else {
      // For page view, line numbers start from offset
      return (currentPage - 1) * pageSize + index + 1;
    }
  };

  return (
    <div className="w-full h-full flex flex-col">
      {/* Controls for log navigation */}
      <div className="flex flex-col gap-2 mb-2 p-2 bg-zinc-100 dark:bg-zinc-800 rounded">
        <div className="flex flex-wrap items-center gap-2">
          {/* Responsive button container */}
          <div className="flex flex-wrap gap-2 min-h-[28px]">
            <Button
              size="sm"
              variant={viewMode === 'tail' ? 'default' : 'outline'}
              onClick={() => handleViewModeChange('tail')}
              disabled={isNavigating}
              className="flex-shrink-0"
            >
              Show End
            </Button>
            <Button
              size="sm"
              variant={viewMode === 'head' ? 'default' : 'outline'}
              onClick={() => handleViewModeChange('head')}
              disabled={isNavigating}
              className="flex-shrink-0"
            >
              Show Beginning
            </Button>
            <Button
              size="sm"
              variant={viewMode === 'page' ? 'default' : 'outline'}
              onClick={() => handleViewModeChange('page')}
              disabled={isNavigating}
              className="flex-shrink-0"
            >
              Page View
            </Button>
          </div>

          <select
            className="px-2 py-1 text-xs border rounded bg-white dark:bg-zinc-700 dark:text-white flex-shrink-0"
            value={pageSize}
            onChange={(e) => setPageSize(Number(e.target.value))}
            disabled={isNavigating}
          >
            <option value="100">100 lines</option>
            <option value="500">500 lines</option>
            <option value="1000">1000 lines</option>
            <option value="5000">5000 lines</option>
          </select>

          {/* Live mode toggle and reload button */}
          <div className="flex items-center gap-2 ml-auto">
            {/* Reload button */}
            <ReloadButton
              onReload={async () => {
                if (mutate) {
                  await mutate();
                }
              }}
              isLoading={isNavigating || isLoading}
              title="Reload logs"
            />

            {/* Live mode toggle - only show when node is running */}
            {isRunning && (
              <button
                onClick={() => setIsLiveMode(!isLiveMode)}
                className={`
                  relative inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium
                  transition-all duration-200 ease-in-out
                  ${isLiveMode 
                    ? 'bg-green-500 text-white shadow-lg shadow-green-500/25' 
                    : 'bg-zinc-200 dark:bg-zinc-700 text-zinc-600 dark:text-zinc-400 hover:bg-zinc-300 dark:hover:bg-zinc-600'
                  }
                `}
              >
                <span className={`
                  inline-block w-2 h-2 rounded-full
                  ${isLiveMode ? 'bg-white animate-pulse' : 'bg-zinc-400 dark:bg-zinc-500'}
                `} />
                <span>LIVE</span>
              </button>
            )}
          </div>
        </div>
        
        {/* Stats line - full width on mobile */}
        <div className="text-xs text-zinc-500 dark:text-zinc-400 flex items-center">
          Showing {lineCount} of {totalLines} lines{' '}
          {isEstimate ? '(estimated)' : ''}{' '}
          {hasMore ? '(more available)' : ''}
        </div>

        {/* Page navigation controls */}
        {viewMode === 'page' && totalLines > 0 && (
          <div className="flex items-center gap-2 mt-2">
            <Button
              size="sm"
              onClick={() => handlePageChange(Math.max(1, currentPage - 1))}
              disabled={currentPage <= 1 || isNavigating}
            >
              Previous
            </Button>
            <span className="text-xs">
              Page {currentPage} of {totalPages}
            </span>
            <Button
              size="sm"
              onClick={() =>
                handlePageChange(Math.min(totalPages, currentPage + 1))
              }
              disabled={currentPage >= totalPages || isNavigating}
            >
              Next
            </Button>
          </div>
        )}

        {/* Jump to line controls */}
        <div className="flex items-center gap-2 mt-2">
          <span className="text-xs">Jump to line:</span>
          <input
            type="number"
            min="1"
            max={totalLines}
            value={jumpToLine}
            onChange={(e) =>
              setJumpToLine(e.target.value === '' ? '' : Number(e.target.value))
            }
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                // Only trigger if the button is not disabled
                if (
                  !isNavigating &&
                  jumpToLine !== '' &&
                  (jumpToLine as number) >= 1 &&
                  (jumpToLine as number) <= totalLines
                ) {
                  handleJumpToLine();
                }
              }
            }}
            className="w-20 px-2 py-1 text-xs border rounded"
            disabled={isNavigating}
          />
          <Button
            size="sm"
            onClick={handleJumpToLine}
            disabled={
              isNavigating ||
              jumpToLine === '' ||
              (jumpToLine as number) < 1 ||
              (jumpToLine as number) > totalLines
            }
          >
            Go
          </Button>
        </div>
      </div>

      {/* Log content with overlay loading indicator when navigating */}
      <div
        ref={logContainerRef}
        className="flex-1 overflow-auto rounded-lg bg-zinc-900 p-4 shadow-md relative"
      >
        {isNavigating && (
          <div className="absolute inset-0 bg-black bg-opacity-20 flex items-center justify-center z-10 pointer-events-none">
            <div className="bg-white dark:bg-zinc-800 rounded-lg p-2 shadow-lg">
              <div className="h-5 w-5 animate-spin rounded-full border-3 border-primary border-t-transparent"></div>
            </div>
          </div>
        )}
        <pre className="h-full font-mono text-sm text-white">
            {lines.map((line, index) => (
            <div
              key={index}
              className="flex hover:bg-zinc-800 px-2 py-0.5 rounded"
            >
              <span
                className="text-zinc-500 mr-4 select-none w-8 text-right flex-shrink-0 self-start"
                data-line-number={getLineNumber(index)}
              >
                {getLineNumber(index)}
              </span>
              <span className="whitespace-pre-wrap break-all flex-grow">
                {line || ' '}
              </span>
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

export default StepLog;
