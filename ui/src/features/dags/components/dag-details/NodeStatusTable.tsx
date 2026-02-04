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
import { components } from '../../../../api/v1/schema';
import NodeStatusTableRow from './NodeStatusTableRow';

/**
 * Props for the NodeStatusTable component
 */
type Props = {
  /** List of nodes to display */
  nodes?: components['schemas']['Node'][];
  /** DAG dagRun details */
  status: components['schemas']['DAGRunDetails'];
  /** DAG file name */
  fileName: string;
  /** Function to open log viewer */
  onViewLog?: (
    stepName: string,
    dagRunId: string,
    node?: components['schemas']['Node']
  ) => void;
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
    <div>
      {/* Desktop view - Table with horizontal scroll for intermediate sizes */}
      <div className="hidden md:block w-full overflow-x-auto">
        <div className="min-w-[900px]">
          <Table className="w-full">
            <TableHeader>
              <TableRow>
                <TableHead className="w-[5%] text-center">
                  No
                </TableHead>
                <TableHead className="w-[20%]">
                  Step Name
                </TableHead>
                <TableHead className="w-[15%]">
                  Command
                </TableHead>
                <TableHead className="w-[15%]">
                  Last Run
                </TableHead>
                <TableHead className="w-[10%] text-center">
                  Status
                </TableHead>
                <TableHead className="w-[35%] min-w-[150px]">
                  Error / Logs
                </TableHead>
                {status?.dagRunId && (
                  <TableHead className="w-[8%] text-center">
                    Actions
                  </TableHead>
                )}
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodes.map((n, idx) => (
                <NodeStatusTableRow
                  key={n.step.name}
                  rownum={idx + 1}
                  node={n}
                  name={fileName}
                  onViewLog={onViewLog}
                  dagRun={status}
                  view="desktop"
                />
              ))}
            </TableBody>
          </Table>
        </div>
      </div>

      {/* Mobile view - Cards */}
      <div className="md:hidden space-y-4">
        {nodes.map((n, idx) => (
          <NodeStatusTableRow
            key={n.step.name}
            rownum={idx + 1}
            node={n}
            name={fileName}
            onViewLog={onViewLog}
            dagRun={status}
            view="mobile"
          />
        ))}
      </div>
    </div>
  );
}

export default NodeStatusTable;
