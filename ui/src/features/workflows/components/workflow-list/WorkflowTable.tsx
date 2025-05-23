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
import { WorkflowDetailsModal } from '../../components/workflow-details';

interface WorkflowTableProps {
  workflows: components['schemas']['WorkflowSummary'][];
}

function WorkflowTable({ workflows }: WorkflowTableProps) {
  const config = useConfig();
  const navigate = useNavigate();
  const [isSmallScreen, setIsSmallScreen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const [selectedWorkflow, setSelectedWorkflow] = useState<{
    name: string;
    workflowId: string;
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

  // Enhanced keyboard navigation for workflows
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // If modal is open, use the existing navigation logic
      if (selectedWorkflow) {
        const currentIndex = workflows.findIndex(
          (item) => item.workflowId === selectedWorkflow.workflowId
        );
        if (currentIndex === -1) return;

        // Navigate with arrow keys
        if (event.key === 'ArrowDown' && currentIndex < workflows.length - 1) {
          // Move to next workflow
          const nextIndex = currentIndex + 1;
          const nextWorkflow = workflows[nextIndex];
          if (nextWorkflow) {
            // Update both selectedWorkflow and selectedIndex
            setSelectedWorkflow({
              name: nextWorkflow.name,
              workflowId: nextWorkflow.workflowId,
            });
            setSelectedIndex(nextIndex);
            scrollToSelectedRow(nextIndex);
          }
        } else if (event.key === 'ArrowUp' && currentIndex > 0) {
          // Move to previous workflow
          const prevIndex = currentIndex - 1;
          const prevWorkflow = workflows[prevIndex];
          if (prevWorkflow) {
            // Update both selectedWorkflow and selectedIndex
            setSelectedWorkflow({
              name: prevWorkflow.name,
              workflowId: prevWorkflow.workflowId,
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
          const newIndex = prev < workflows.length - 1 ? prev + 1 : prev;
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
        const selectedItem = workflows[selectedIndex];
        if (selectedItem) {
          setSelectedWorkflow({
            name: selectedItem.name,
            workflowId: selectedItem.workflowId,
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
  }, [selectedWorkflow, workflows, selectedIndex]);

  // Initialize selection when workflows change
  useEffect(() => {
    if (workflows.length > 0 && selectedIndex === -1) {
      setSelectedIndex(0);
    } else if (selectedIndex >= workflows.length) {
      setSelectedIndex(workflows.length - 1);
    }
  }, [workflows, selectedIndex]);

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
    // If workflow hasn't started yet, return dash
    if (!startedAt) {
      return '-';
    }

    if (!finishedAt) {
      // If workflow is still running, calculate duration from start until now
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
      <h3 className="text-lg font-medium text-gray-900 mb-2">
        No workflows found
      </h3>
      <p className="text-sm text-gray-500 text-center max-w-md mb-4">
        There are no workflows matching your current filters. Try adjusting your
        search criteria or date range.
      </p>
    </div>
  );

  // If there are no workflows, show empty state
  if (workflows.length === 0) {
    return <EmptyState />;
  }

  // Card view for small screens - Direct navigation without modal
  if (isSmallScreen) {
    return (
      <div className="space-y-2">
        {workflows.map((workflow, index) => (
          <div
            key={workflow.workflowId}
            className={`p-3 rounded-lg border min-h-[80px] flex flex-col ${
              selectedIndex === index
                ? 'bg-primary/10 border-primary'
                : 'bg-card border-border'
            } cursor-pointer shadow-sm`}
            onClick={(e) => {
              // Navigate directly to workflow page with correct URL pattern
              if (e.metaKey || e.ctrlKey) {
                // Open in new tab if Cmd/Ctrl is pressed
                window.open(
                  `/workflows/${workflow.name}/${workflow.workflowId}`,
                  '_blank'
                );
              } else {
                // Use React Router for SPA navigation
                navigate(`/workflows/${workflow.name}/${workflow.workflowId}`);
              }
            }}
          >
            {/* Header with name and status */}
            <div className="flex justify-between items-start mb-2">
              <div className="font-medium text-sm">{workflow.name}</div>
              <StatusChip status={workflow.status} size="xs">
                {workflow.statusLabel}
              </StatusChip>
            </div>

            {/* Workflow ID */}
            <div className="text-xs font-mono text-muted-foreground mb-2">
              {workflow.workflowId}
            </div>

            {/* Timestamps */}
            <div className="space-y-1 text-xs mt-2">
              <div className="flex justify-between items-center">
                <div>
                  <span className="text-muted-foreground">Queued: </span>
                  {workflow.queuedAt || '-'}
                </div>
                <div>
                  <span className="text-muted-foreground">Started: </span>
                  {workflow.startedAt || '-'}
                </div>
              </div>
              <div className="text-left flex items-center gap-1.5">
                <span className="text-muted-foreground">Duration: </span>
                <span className="flex items-center gap-1">
                  {calculateDuration(workflow.startedAt, workflow.finishedAt)}
                  {workflow.status === 1 && workflow.startedAt && (
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
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Workflow Name
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Workflow ID
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Status
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              <div>Queued At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              <div>Started At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Duration
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {workflows.map((workflow, index) => (
            <TableRow
              key={workflow.workflowId}
              className={`cursor-pointer ${
                selectedIndex === index
                  ? 'bg-primary/10 hover:bg-primary/15 border-l-4 border-primary border-b-0 border-t-0'
                  : 'hover:bg-muted/50 border-0'
              }`}
              style={{ fontSize: '0.8125rem' }}
              onClick={() => {
                setSelectedIndex(index);
                setSelectedWorkflow({
                  name: workflow.name,
                  workflowId: workflow.workflowId,
                });
              }}
            >
              <TableCell className="py-1 px-2 font-medium">
                {workflow.name}
              </TableCell>
              <TableCell className="py-1 px-2 font-mono text-slate-600">
                {workflow.workflowId}
              </TableCell>
              <TableCell className="py-1 px-2">
                <div className="flex items-center">
                  <StatusChip status={workflow.status} size="xs">
                    {workflow.statusLabel}
                  </StatusChip>
                </div>
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {workflow.queuedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                {workflow.startedAt || '-'}
              </TableCell>
              <TableCell className="py-1 px-2 text-left">
                <div className="flex items-center gap-1">
                  {calculateDuration(workflow.startedAt, workflow.finishedAt)}
                  {workflow.status === 1 && workflow.startedAt && (
                    <span className="inline-block w-2 h-2 rounded-full bg-lime-500 animate-pulse" />
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {/* Workflow Details Modal */}
      {selectedWorkflow && (
        <WorkflowDetailsModal
          name={selectedWorkflow.name}
          workflowId={selectedWorkflow.workflowId}
          isOpen={!!selectedWorkflow}
          onClose={() => {
            setSelectedWorkflow(null);
            // Don't reset selectedIndex when closing modal
            // This keeps the row highlighted after closing
          }}
        />
      )}
    </div>
  );
}

export default WorkflowTable;
