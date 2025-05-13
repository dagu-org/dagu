import { components } from '../../../../api/v2/schema';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../../components/ui/table';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';

interface WorkflowTableProps {
  workflows: components['schemas']['WorkflowSummary'][];
}

function WorkflowTable({ workflows }: WorkflowTableProps) {
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
              Started At
            </TableHead>
            <TableHead className="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-xs">
              Finished At
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
              <TableCell className="text-xs">
                {dayjs(workflow.startedAt).format('YYYY-MM-DD HH:mm:ss')}
              </TableCell>
              <TableCell className="text-xs">
                {workflow.finishedAt
                  ? dayjs(workflow.finishedAt).format('YYYY-MM-DD HH:mm:ss')
                  : '-'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export default WorkflowTable;
