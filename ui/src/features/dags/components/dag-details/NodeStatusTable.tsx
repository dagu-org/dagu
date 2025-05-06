/**
 * NodeStatusTable component displays a table of node execution statuses.
 *
 * @module features/dags/components/dag-details
 */
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { components } from '../../../../api/v2/schema';
import NodeStatusTableRow from './NodeStatusTableRow';

/**
 * Props for the NodeStatusTable component
 */
type Props = {
  /** List of nodes to display */
  nodes?: components['schemas']['Node'][];
  /** DAG run details */
  status: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileName: string;
  /** Function to open log viewer */
  onViewLog?: (stepName: string, requestId: string) => void;
};

/**
 * NodeStatusTable displays execution status information for all nodes in a DAG run
 */
function NodeStatusTable({ nodes, status, fileName, onViewLog }: Props) {
  // Don't render if there are no nodes
  if (!nodes || !nodes.length) {
    return null;
  }

  return (
    <div className="w-full overflow-x-auto">
      <Table className="table-fixed w-full">
        <TableHeader className="bg-slate-50 dark:bg-slate-800">
          <TableRow className="border-b border-slate-200 dark:border-slate-700">
            <TableHead className="w-[5%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              No
            </TableHead>
            <TableHead className="w-[20%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Step Name
            </TableHead>
            <TableHead className="w-[20%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Command
            </TableHead>
            <TableHead className="w-[30%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Last Run
            </TableHead>
            <TableHead className="w-[10%] py-3 text-center text-sm font-semibold text-slate-700 dark:text-slate-300">
              Status
            </TableHead>
            <TableHead className="w-[20%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Error
            </TableHead>
            <TableHead className="w-[20%] py-3 text-center text-sm font-semibold text-slate-700 dark:text-slate-300">
              Log
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((n, idx) => (
            <NodeStatusTableRow
              key={n.step.name}
              rownum={idx + 1}
              node={n}
              requestId={status.requestId}
              name={fileName}
              onViewLog={onViewLog}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export default NodeStatusTable;
