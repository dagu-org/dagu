/**
 * StepLog component displays the execution log for a specific step in a DAG run.
 *
 * @module features/dags/components/dag-execution
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Download } from 'lucide-react';
import { components, NodeStatus, Stream } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import { Input } from '../../../../components/ui/input';
import { ReloadButton } from '../../../../components/ui/reload-button';
import { Switch } from '../../../../components/ui/switch';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { TOKEN_KEY } from '../../../../contexts/AuthContext';
import { useUserPreferences } from '../../../../contexts/UserPreference';
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
  stream?: Stream;
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
  stream = Stream.stdout,
  node,
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const { preferences, updatePreference } = useUserPreferences();
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

  // Determine if this is a sub DAG-run - check both rootDAGRunId AND rootDAGRunName
  const isSubDAGRun =
    dagRun &&
    dagRun.rootDAGRunId &&
    dagRun.rootDAGRunName &&
    dagRun.rootDAGRunId !== dagRun.dagRunId;

  // Build query params based on view mode
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const tail = viewMode === 'tail' ? pageSize : undefined;
  const head = viewMode === 'head' ? pageSize : undefined;
  const offset = viewMode === 'page' ? (currentPage - 1) * pageSize + 1 : undefined;
  const limit = viewMode === 'page' ? pageSize : undefined;

  // SWR options shared by both queries - memoized to prevent unnecessary re-renders
  const swrOptions = React.useMemo(
    () => ({
      refreshInterval: isLiveMode && isRunning ? 2000 : 0,
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 1000,
    }),
    [isLiveMode, isRunning]
  );

  // Fetch sub-DAG-run step log (only when isSubDAGRun is true)
  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/log',
    {
      params: {
        query: {
          remoteNode,
          stream,
          tail,
          head,
          offset,
          limit,
        },
        path: {
          name: dagRun?.rootDAGRunName as string,
          dagRunId: dagRun?.rootDAGRunId as string,
          subDAGRunId: dagRun?.dagRunId as string,
          stepName,
        },
      },
    },
    { ...swrOptions, isPaused: () => !isSubDAGRun }
  );

  // Fetch regular DAG-run step log (only when isSubDAGRun is false)
  const dagRunQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/steps/{stepName}/log',
    {
      params: {
        query: {
          remoteNode,
          stream,
          tail,
          head,
          offset,
          limit,
        },
        path: {
          name: dagName,
          dagRunId,
          stepName,
        },
      },
    },
    { ...swrOptions, isPaused: () => !!isSubDAGRun }
  );

  // Use the appropriate query based on whether this is a sub-DAG-run
  const { data, isLoading, error, mutate } = isSubDAGRun ? subDAGQuery : dagRunQuery;

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
            element.classList.add('bg-primary/100', 'bg-opacity-20');
            setTimeout(() => {
              element.classList.remove('bg-primary/100', 'bg-opacity-20');
            }, 2000);
            break;
          }
        }
      }, 500);
    }
  };

  const handleDownload = useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY);
    const endpoint = isSubDAGRun
      ? `/api/v2/dag-runs/${dagRun?.rootDAGRunName}/${dagRun?.rootDAGRunId}/sub-dag-runs/${dagRun?.dagRunId}/steps/${stepName}/log/download`
      : `/api/v2/dag-runs/${dagName}/${dagRunId}/steps/${stepName}/log/download`;

    const url = new URL(endpoint, window.location.origin);
    url.searchParams.set('remoteNode', remoteNode);
    url.searchParams.set('stream', stream);

    try {
      const response = await fetch(url.toString(), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });

      if (!response.ok) {
        throw new Error(`Download failed: ${response.statusText}`);
      }

      const blob = await response.blob();
      const filename =
        response.headers
          .get('Content-Disposition')
          ?.match(/filename="(.+)"/)?.[1] ||
        `${dagName}-${dagRunId}-${stepName}-${stream}.log`;

      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = filename;
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (err) {
      console.error('Download failed:', err);
    }
  }, [dagName, dagRunId, stepName, stream, dagRun, isSubDAGRun, remoteNode]);

  // Show loading indicator only on initial load
  if (isLoading && !cachedData && isInitialLoad.current) {
    return <LoadingIndicator />;
  }

  // Use cached data if available, otherwise use current data
  const logData = (data || cachedData) as LogWithPagination;

  // Handle error state (but not 404 - that just means no log file exists yet)
  if (error && !logData) {
    const isNotFound = error.message?.includes('not found');
    if (!isNotFound) {
      return (
        <div className="w-full h-full flex items-center justify-center">
          <div className="text-error">
            Error loading log data: {error.message || 'Unknown error'}
          </div>
        </div>
      );
    }
  }

  // Process log data
  const content =
    logData?.content.replace(new RegExp(ANSI_CODES_REGEX, 'g'), '') || '';
  const totalLines = logData?.totalLines || 0;
  const hasMore = logData?.hasMore || false;
  const isEstimate = logData?.isEstimate || false;

  // Split content into lines, removing trailing empty line from trailing newline
  const rawLines = content ? content.split('\n') : ['<No log output>'];
  const lines = rawLines[rawLines.length - 1] === '' ? rawLines.slice(0, -1) : rawLines;

  // API may count trailing newline as extra line; use lines.length if within 1
  const effectiveTotalLines = (totalLines - lines.length <= 1) ? lines.length : totalLines;

  const totalPages = calculateTotalPages(effectiveTotalLines, pageSize);

  const getLineNumber = (index: number): number => {
    if (viewMode === 'tail') {
      return Math.max(1, effectiveTotalLines - lines.length + 1) + index;
    } else if (viewMode === 'head') {
      return index + 1;
    } else {
      return (currentPage - 1) * pageSize + index + 1;
    }
  };

  return (
    <div className="w-full h-full flex flex-col">
      {/* Controls for log navigation */}
      <div className="flex flex-col gap-2 mb-2 bg-muted rounded">
        <div className="flex flex-wrap items-center gap-2">
          {/* Responsive button container */}
          <div className="flex flex-wrap gap-1">
            <Button
              size="sm"
              variant={viewMode === 'tail' ? 'primary' : 'default'}
              onClick={() => handleViewModeChange('tail')}
              disabled={isNavigating}
            >
              Show End
            </Button>
            <Button
              size="sm"
              variant={viewMode === 'head' ? 'primary' : 'default'}
              onClick={() => handleViewModeChange('head')}
              disabled={isNavigating}
            >
              Show Beginning
            </Button>
            <Button
              size="sm"
              variant={viewMode === 'page' ? 'primary' : 'default'}
              onClick={() => handleViewModeChange('page')}
              disabled={isNavigating}
            >
              Page View
            </Button>
          </div>

          <select
            className="h-7 px-2 text-xs border border-border rounded-md bg-surface text-foreground flex-shrink-0 focus:outline-none focus:ring-1 focus:ring-ring"
            value={pageSize}
            onChange={(e) => setPageSize(Number(e.target.value))}
            disabled={isNavigating}
          >
            <option value="100">100 lines</option>
            <option value="500">500 lines</option>
            <option value="1000">1000 lines</option>
            <option value="5000">5000 lines</option>
            <option value="10000">10000 lines</option>
          </select>

          {/* Wrap toggle, Live mode toggle and reload button */}
          <div className="flex items-center gap-2 ml-auto">
            {/* Wrap toggle */}
            <div className="flex items-center gap-1.5">
              <span className="text-xs text-muted-foreground">Wrap</span>
              <Switch
                checked={preferences.logWrap}
                onCheckedChange={(checked) => updatePreference('logWrap', checked)}
              />
            </div>

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

            {/* Download button */}
            <Button
              size="sm"
              variant="outline"
              onClick={handleDownload}
              disabled={isNavigating}
              title="Download full log"
            >
              <Download className="h-4 w-4" />
            </Button>

            {/* Live mode toggle - only show when node is running */}
            {isRunning && (
              <Button
                size="sm"
                variant={isLiveMode ? 'primary' : 'default'}
                onClick={() => setIsLiveMode(!isLiveMode)}
              >
                <span
                  className={`inline-block w-2 h-2 rounded-full ${isLiveMode ? 'bg-white animate-pulse' : 'bg-muted-foreground'}`}
                />
                LIVE
              </Button>
            )}
          </div>
        </div>

        {/* Stats line - full width on mobile */}
        <div className="text-xs text-muted-foreground flex items-center">
          Showing {lines.length} of {effectiveTotalLines} lines{' '}
          {isEstimate ? '(estimated)' : ''} {hasMore ? '(more available)' : ''}
        </div>

        {/* Page navigation controls */}
        {viewMode === 'page' && effectiveTotalLines > 0 && (
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
          <span className="text-xs text-muted-foreground">Jump to line:</span>
          <Input
            type="number"
            min={1}
            max={effectiveTotalLines}
            value={jumpToLine}
            onChange={(e) =>
              setJumpToLine(e.target.value === '' ? '' : Number(e.target.value))
            }
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                if (
                  !isNavigating &&
                  jumpToLine !== '' &&
                  (jumpToLine as number) >= 1 &&
                  (jumpToLine as number) <= effectiveTotalLines
                ) {
                  handleJumpToLine();
                }
              }
            }}
            className="w-20 h-7 text-xs"
            disabled={isNavigating}
          />
          <Button
            size="sm"
            onClick={handleJumpToLine}
            disabled={
              isNavigating ||
              jumpToLine === '' ||
              (jumpToLine as number) < 1 ||
              (jumpToLine as number) > effectiveTotalLines
            }
          >
            Go
          </Button>
        </div>
      </div>

      {/* Log content with overlay loading indicator when navigating */}
      <div
        ref={logContainerRef}
        className={`flex-1 rounded-lg bg-slate-800 pt-4 pr-4 pb-4 relative ${preferences.logWrap ? 'overflow-auto' : 'overflow-x-auto overflow-y-auto'}`}
      >
        {isNavigating && (
          <div className="absolute inset-0 bg-black bg-opacity-20 flex items-center justify-center z-10 pointer-events-none">
            <div className="bg-card rounded-lg p-2">
              <div className="h-5 w-5 animate-spin rounded-full border-3 border-primary border-t-transparent"></div>
            </div>
          </div>
        )}
        <pre className={`h-full font-mono text-sm text-slate-100 log-content ${preferences.logWrap ? '' : 'min-w-max'}`}>
          {lines.map((line, index) => (
            <div key={index} className="flex pr-2 py-0.5">
              <span
                className="text-slate-500 mr-4 select-none w-14 text-right flex-shrink-0 self-start sticky left-0 bg-slate-800 pl-4 pr-2 z-10"
                data-line-number={getLineNumber(index)}
              >
                {getLineNumber(index)}
              </span>
              <span className={`flex-grow select-text cursor-text ${preferences.logWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'}`}>
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
