import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../../components/ui/table';
import { useConfig } from '../../../../contexts/ConfigContext';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import { DAGRunDetailsModal } from '../dag-run-details';

interface DAGRunTableProps {
  dagRuns: components['schemas']['DAGRunSummary'][];
}

function DAGRunTable({ dagRuns }: DAGRunTableProps) {
  const config = useConfig();
  const navigate = useNavigate();
  const [isSmallScreen, setIsSmallScreen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const [selectedDAGRun, setSelectedDAGRun] = useState<{
    name: string;
    dagRunId: string;
  } | null>(null);
  const tableRef = useRef<HTMLDivElement>(null);

  // Check screen size on mount and when window resizes
  useEffect(() => {
    const checkScreenSize = () => {
      setIsSmallScreen(window.innerWidth < 768); // 768px is typically md breakpoint
    };

    // Initial check
    checkScreenSize();

    // Add event listener
    window.addEventListener('resize', checkScreenSize);

    // Cleanup
    return () => window.removeEventListener('resize', checkScreenSize);
  }, []);

  // Enhanced keyboard navigation for dagRuns
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // If modal is open, use the existing navigation logic
      if (selectedDAGRun) {
        const currentIndex = dagRuns.findIndex(
          (item) => item.dagRunId === selectedDAGRun.dagRunId
        );
        if (currentIndex === -1) return;

        // Navigate with arrow keys
        if (event.key === 'ArrowDown' && currentIndex < dagRuns.length - 1) {
          // Move to next DAG-run
          const nextIndex = currentIndex + 1;
          const nextDAGRun = dagRuns[nextIndex];
          if (nextDAGRun) {
            // Update both selectedDAGRun and selectedIndex
            setSelectedDAGRun({
              name: nextDAGRun.name,
              dagRunId: nextDAGRun.dagRunId,
            });
            setSelectedIndex(nextIndex);
            scrollToSelectedRow(nextIndex);
          }
        } else if (event.key === 'ArrowUp' && currentIndex > 0) {
          // Move to previous DAG-run
          const prevIndex = currentIndex - 1;
          const prevDAGRun = dagRuns[prevIndex];
          if (prevDAGRun) {
            // Update both selectedDAGRun and selectedIndex
            setSelectedDAGRun({
              name: prevDAGRun.name,
              dagRunId: prevDAGRun.dagRunId,
            });
            setSelectedIndex(prevIndex);
            scrollToSelectedRow(prevIndex);
          }
        }
        return;
      }

      // If no modal is open, handle table row selection
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setSelectedIndex((prev) => {
          const newIndex = prev < dagRuns.length - 1 ? prev + 1 : prev;
          scrollToSelectedRow(newIndex);
          return newIndex;
        });
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        setSelectedIndex((prev) => {
          const newIndex = prev > 0 ? prev - 1 : prev === -1 ? 0 : prev;
          scrollToSelectedRow(newIndex);
          return newIndex;
        });
      } else if (event.key === 'Enter' && selectedIndex >= 0) {
        // Open modal when Enter is pressed on selected row
        const selectedItem = dagRuns[selectedIndex];
        if (selectedItem) {
          setSelectedDAGRun({
            name: selectedItem.name,
            dagRunId: selectedItem.dagRunId,
          });
        }
      }
    };

    // Helper function to scroll to the selected row
    const scrollToSelectedRow = (index: number) => {
      if (index >= 0 && tableRef.current) {
        const rows = tableRef.current.querySelectorAll('tbody tr');
        if (rows[index]) {
          rows[index].scrollIntoView({
            behavior: 'smooth',
            block: 'nearest',
          });
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [selectedDAGRun, dagRuns, selectedIndex]);

  // Initialize selection when dagRuns change
  useEffect(() => {
    if (dagRuns.length > 0 && selectedIndex === -1) {
      setSelectedIndex(0);
    } else if (selectedIndex >= dagRuns.length) {
      setSelectedIndex(dagRuns.length - 1);
    }
  }, [dagRuns, selectedIndex]);

  // Format timezone information for display
  const getTimezoneInfo = (): string => {
    if (config.tzOffsetInSec === undefined) return 'Local Timezone';

    // Convert seconds to hours and minutes
    const offsetInMinutes = config.tzOffsetInSec / 60;
    const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
    const minutes = Math.abs(offsetInMinutes) % 60;

    // Format with sign and padding
    const sign = offsetInMinutes >= 0 ? '+' : '-';
    const formattedHours = hours.toString().padStart(2, '0');
    const formattedMinutes = minutes.toString().padStart(2, '0');

    return `${sign}${formattedHours}:${formattedMinutes}`;
  };

  // Calculate duration between start and finish times
  const calculateDuration = (
    startedAt: string,
    finishedAt: string | null
  ): string => {
    // If DAG-run hasn't started yet, return dash
    if (!startedAt) {
      return '-';
    }

    if (!finishedAt) {
      // If DAG-run is still running, calculate duration from start until now
      const start = dayjs(startedAt);
      const now = dayjs();
      const durationMs = now.diff(start);
      return formatDuration(durationMs);
    }

    const start = dayjs(startedAt);
    const end = dayjs(finishedAt);
    const durationMs = end.diff(start);
    return formatDuration(durationMs);
  };

  // Format duration in a human-readable format
  const formatDuration = (durationMs: number): string => {
    const seconds = Math.floor(durationMs / 1000);

    if (seconds < 60) {
      return `${seconds}s`;
    }

    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    if (minutes < 60) {
      return `${minutes}m ${remainingSeconds}s`;
    }

    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;

    return `${hours}h ${remainingMinutes}m ${remainingSeconds}s`;
  };

  const timezoneInfo = getTimezoneInfo();

  // Empty state component
  const EmptyState = () => (
    <div className="flex flex-col items-center justify-center py-12 px-4 border rounded-md bg-white">
      <div className="text-6xl mb-4">🔍</div>
      <h3 className="text-lg font-normal text-gray-900 mb-2">
        No DAG runs found
      </h3>
      <p className="text-sm text-gray-500 text-center max-w-md mb-4">
        There are no DAG runs matching your current filters. Try adjusting your
        search criteria or date range.
      </p>
    </div>
  );

  // If there are no DAG runs, show empty state
  if (dagRuns.length === 0) {
    return <EmptyState />;
  }

  // Card view for small screens - Direct navigation without modal
  if (isSmallScreen) {
    return (
      <div className="space-y-2">
        {dagRuns.map((dagRun, index) => (
          <div
            key={dagRun.dagRunId}
            className={`p-3 rounded-lg border min-h-[80px] flex flex-col ${
              selectedIndex === index
                ? 'bg-primary/10 border-primary'
                : 'bg-card border-border'
            } cursor-pointer shadow-sm`}
            onClick={(e) => {
              // Navigate directly to DAG-run page with correct URL pattern
              if (e.metaKey || e.ctrlKey) {
                // Open in new tab if Cmd/Ctrl is pressed
                window.open(
                  `/dag-runs/${dagRun.name}/${dagRun.dagRunId}`,
                  '_blank'
                );
              } else {
                // Use React Router for SPA navigation
                navigate(`/dag-runs/${dagRun.name}/${dagRun.dagRunId}`);
              }
            }}
          >
            {/* Header with name and status */}
            <div className="flex justify-between items-start mb-2">
              <div className="font-normal text-sm">{dagRun.name}</div>
              <StatusChip status={dagRun.status} size="xs">
                {dagRun.statusLabel}
              </StatusChip>
            </div>

            {/* DAG-run ID */}
            <div className="text-xs font-mono text-muted-foreground mb-2">
              {dagRun.dagRunId}
            </div>

            {/* Timestamps */}
            <div className="space-y-1 text-xs mt-2">
              <div className="flex justify-between items-center">
                <div>
                  <span className="text-muted-foreground">Queued: </span>
                  {dagRun.queuedAt || '-'}
                </div>
                <div>
                  <span className="text-muted-foreground">Started: </span>
                  {dagRun.startedAt || '-'}
                </div>
              </div>
              <div className="text-left flex items-center gap-1.5">
                <span className="text-muted-foreground">Duration: </span>
                <span className="flex items-center gap-1">
                  {calculateDuration(dagRun.startedAt, dagRun.finishedAt)}
                  {dagRun.status === 1 && dagRun.startedAt && (
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-lime-500 animate-pulse" />
                  )}
                </span>
              </div>
            </div>

            {/* Timezone info */}
            <div className="text-[10px] text-muted-foreground text-right pt-1">
              {timezoneInfo}
            </div>
          </div>
        ))}
      </div>
    );
  }

  // Table view for larger screens
  return (
    <div className="border rounded-md bg-white" ref={tableRef}>
      <Table className="w-full text-xs">
        <TableHeader>
          <TableRow>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              DAG Name
            </TableHead>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Run ID
            </TableHead>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Status
            </TableHead>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              <div>Queued At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              <div>Started At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Duration
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {dagRuns.map((dagRun, index) => (
            <TableRow
              key={dagRun.dagRunId}
              className={`cursor-pointer ${
                selectedIndex === index
                  ? 'bg-primary/10 hover:bg-primary/15 border-l-4 border-primary border-b-0 border-t-0'
                  : 'hover:bg-muted/50 border-0'
              }`}
              style={{ fontSize: '0.8125rem' }}
              onClick={(e) => {
                if (e.ctrlKey || e.metaKey) {
                  // Open in new tab
                  window.open(`/dag-runs/${dagRun.name}/${dagRun.dagRunId}`, '_blank');
                } else {
                  // Open modal
                  setSelectedIndex(index);
                  setSelectedDAGRun({
                    name: dagRun.name,
                    dagRunId: dagRun.dagRunId,
                  });
                }
              }}
            >
              <TableCell className="py-1 px-2 font-normal">
                {dagRun.name}
              </TableCell>
              <TableCell className="py-1 px-2 font-mono text-muted-foreground">
                {dagRun.dagRunId}
              </TableCell>
              <TableCell className="py-1 px-2">
                <div className="flex items-center">
                  <StatusChip status={dagRun.status} size="xs">
                    {dagRun.statusLabel}
                  </StatusChip>
                </div>
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {dagRun.queuedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {dagRun.startedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                <div className="flex items-center gap-1">
                  {calculateDuration(dagRun.startedAt, dagRun.finishedAt)}
                  {dagRun.status === 1 && dagRun.startedAt && (
                    <span className="inline-block w-2 h-2 rounded-full bg-lime-500 animate-pulse" />
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {/* DAG-run Details Modal */}
      {selectedDAGRun && (
        <DAGRunDetailsModal
          name={selectedDAGRun.name}
          dagRunId={selectedDAGRun.dagRunId}
          isOpen={!!selectedDAGRun}
          onClose={() => {
            setSelectedDAGRun(null);
            // Don't reset selectedIndex when closing modal
            // This keeps the row highlighted after closing
          }}
        />
      )}
    </div>
  );
}

export default DAGRunTable;
