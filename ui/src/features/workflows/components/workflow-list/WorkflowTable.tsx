import { useEffect, useState } from 'react';
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
import StatusChip from '../../../../ui/StatusChip';
import WorkflowCard from './WorkflowCard';

interface WorkflowTableProps {
  workflows: components['schemas']['WorkflowSummary'][];
}

function WorkflowTable({ workflows }: WorkflowTableProps) {
  const config = useConfig();
  const [isSmallScreen, setIsSmallScreen] = useState(false);

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

  const timezoneInfo = getTimezoneInfo();

  // Card view for small screens
  if (isSmallScreen) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {workflows.map((workflow) => (
          <WorkflowCard
            key={workflow.workflowId}
            workflow={workflow}
            timezoneInfo={timezoneInfo}
          />
        ))}
      </div>
    );
  }

  // Table view for larger screens
  return (
    <div className="border rounded-md bg-white">
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
              <div>Started At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              <div>Finished At</div>
              <div className="text-[10px] text-muted-foreground font-normal">
                {timezoneInfo}
              </div>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {workflows.map((workflow) => (
            <TableRow key={workflow.workflowId}>
              <TableCell className="text-xs">{workflow.name}</TableCell>
              <TableCell className="text-xs">{workflow.workflowId}</TableCell>
              <TableCell className="text-xs">
                <StatusChip status={workflow.status} size="xs">
                  {workflow.statusLabel}
                </StatusChip>
              </TableCell>
              <TableCell className="text-xs">{workflow.startedAt}</TableCell>
              <TableCell className="text-xs">
                {workflow.finishedAt || '-'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export default WorkflowTable;
