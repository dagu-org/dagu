import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
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
import { StepDetailsTooltip } from './StepDetailsTooltip';

interface DAGRunTableProps {
  dagRuns: components['schemas']['DAGRunSummary'][];
  selectedDAGRun?: { name: string; dagRunId: string } | null;
  onSelectDAGRun?: (dagRun: { name: string; dagRunId: string } | null) => void;
}

function DAGRunTable({
  dagRuns,
  selectedDAGRun = null,
  onSelectDAGRun,
}: DAGRunTableProps) {
  const config = useConfig();
  const navigate = useNavigate();
  const [isSmallScreen, setIsSmallScreen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
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

  // Update selectedIndex when selectedDAGRun changes from parent
  useEffect(() => {
    if (selectedDAGRun) {
      const index = dagRuns.findIndex(
        (item) => item.dagRunId === selectedDAGRun.dagRunId
      );
      if (index !== -1 && index !== selectedIndex) {
        setSelectedIndex(index);
        scrollToSelectedRow(index);
      }
    }
  }, [selectedDAGRun, dagRuns]);

  // Keyboard navigation - works both when panel is open and closed
  useEffect(() => {
    if (!onSelectDAGRun) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      // Find current index based on selectedDAGRun or selectedIndex
      const currentIdx = selectedDAGRun
        ? dagRuns.findIndex((item) => item.dagRunId === selectedDAGRun.dagRunId)
        : selectedIndex;

      if (event.key === 'ArrowDown') {
        event.preventDefault();
        const newIndex = currentIdx < dagRuns.length - 1 ? currentIdx + 1 : currentIdx;
        if (newIndex !== currentIdx) {
          setSelectedIndex(newIndex);
          scrollToSelectedRow(newIndex);
          // If panel is open, navigate to new item
          if (selectedDAGRun && dagRuns[newIndex]) {
            onSelectDAGRun({
              name: dagRuns[newIndex].name,
              dagRunId: dagRuns[newIndex].dagRunId,
            });
          }
        }
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        let newIndex: number;
        if (currentIdx > 0) {
          newIndex = currentIdx - 1;
        } else if (currentIdx === -1) {
          newIndex = 0;
        } else {
          newIndex = currentIdx;
        }
        if (newIndex !== currentIdx || currentIdx === -1) {
          setSelectedIndex(newIndex);
          scrollToSelectedRow(newIndex);
          const dagRunAtNewIndex = dagRuns[newIndex];
          if (selectedDAGRun && dagRunAtNewIndex) {
            onSelectDAGRun({
              name: dagRunAtNewIndex.name,
              dagRunId: dagRunAtNewIndex.dagRunId,
            });
          }
        }
      } else if (event.key === 'Enter' && !selectedDAGRun && currentIdx >= 0) {
        const selectedItem = dagRuns[currentIdx];
        if (selectedItem) {
          onSelectDAGRun({
            name: selectedItem.name,
            dagRunId: selectedItem.dagRunId,
          });
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [selectedDAGRun, dagRuns, selectedIndex, onSelectDAGRun]);

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
    finishedAt: string | null,
    status: number
  ): string => {
    // If DAG-run hasn't started yet, return dash
    if (!startedAt) {
      return '-';
    }

    // Only calculate duration dynamically for running DAGs
    if (status === Status.Running && !finishedAt) {
      // If DAG-run is still running, calculate duration from start until now
      const start = dayjs(startedAt);
      const now = dayjs();
      const durationMs = now.diff(start);
      return formatDuration(durationMs);
    }

    // For finished DAGs, use the static duration
    if (finishedAt) {
      const start = dayjs(startedAt);
      const end = dayjs(finishedAt);
      const durationMs = end.diff(start);
      return formatDuration(durationMs);
    }

    // For non-running DAGs without a finish time, return dash
    return '-';
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
    <div className="flex flex-col items-center justify-center py-12 px-4 border rounded-md bg-card">
      <div className="text-6xl mb-4">🔍</div>
      <h3 className="text-lg font-normal text-foreground mb-2">
        No DAG runs found
      </h3>
      <p className="text-sm text-muted-foreground text-center max-w-md mb-4">
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
            className={`p-3 rounded-lg border border-l-4 min-h-[80px] flex flex-col bg-card border-border ${selectedIndex === index ? 'border-l-border' : 'border-l-transparent'} ${dagRun.status === Status.Running ? 'animate-running-row' : ''} cursor-pointer`}
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
              <StepDetailsTooltip dagRun={dagRun}>
                <div className="flex items-center">
                  <StatusChip status={dagRun.status} size="xs">
                    {dagRun.statusLabel}
                  </StatusChip>
                </div>
              </StepDetailsTooltip>
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
                  {calculateDuration(
                    dagRun.startedAt,
                    dagRun.finishedAt,
                    dagRun.status
                  )}
                  {dagRun.status === Status.Running && dagRun.startedAt && (
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
                  )}
                </span>
              </div>
              {dagRun.workerId && (
                <div className="text-left">
                  <span className="text-muted-foreground">Worker: </span>
                  {dagRun.workerId}
                </div>
              )}
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
    <div ref={tableRef}>
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
            <TableHead className="text-muted-foreground h-10 px-2 text-left align-middle font-normal whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Worker
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {dagRuns.map((dagRun, index) => (
            <TableRow
              key={dagRun.dagRunId}
              className={`cursor-pointer hover:bg-muted/50 border-l-4 ${
                selectedIndex === index ? 'border-l-border' : 'border-l-transparent'
              } ${dagRun.status === Status.Running ? 'animate-running-row' : ''}`}
              style={{ fontSize: '0.8125rem' }}
              onClick={(e) => {
                if (e.ctrlKey || e.metaKey) {
                  // Open in new tab
                  window.open(
                    `/dag-runs/${dagRun.name}/${dagRun.dagRunId}`,
                    '_blank'
                  );
                } else if (isSmallScreen) {
                  // On small screens, navigate to full page
                  navigate(`/dag-runs/${dagRun.name}/${dagRun.dagRunId}`);
                } else if (onSelectDAGRun) {
                  // Select the DAG run
                  setSelectedIndex(index);
                  onSelectDAGRun({
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
                <StepDetailsTooltip dagRun={dagRun}>
                  <div className="flex items-center">
                    <StatusChip status={dagRun.status} size="xs">
                      {dagRun.statusLabel}
                    </StatusChip>
                  </div>
                </StepDetailsTooltip>
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {dagRun.queuedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {dagRun.startedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                <div className="flex items-center gap-1">
                  {calculateDuration(
                    dagRun.startedAt,
                    dagRun.finishedAt,
                    dagRun.status
                  )}
                  {dagRun.status === Status.Running && dagRun.startedAt && (
                    <span className="inline-block w-2 h-2 rounded-full bg-success animate-pulse" />
                  )}
                </div>
              </TableCell>
              <TableCell className="py-1 px-2 text-muted-foreground">
                {dagRun.workerId || '-'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export default DAGRunTable;
