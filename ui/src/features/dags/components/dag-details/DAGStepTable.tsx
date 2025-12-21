/**
 * DAGStepTable component displays a table of steps in a DAG.
 *
 * @module features/dags/components/dag-details
 */
import { components } from '../../../../api/v2/schema';
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../../components/ui/table';
import DAGStepTableRow from './DAGStepTableRow';

/**
 * Props for the DAGStepTable component
 */
type Props = {
  /** List of steps to display */
  steps: components['schemas']['Step'][];
  /** Optional title for the table */
  title?: string;
};

/**
 * DAGStepTable displays a table of steps in a DAG with their properties
 */
function DAGStepTable({ steps }: Props) {
  // Don't render if there are no steps
  if (!steps.length) {
    return null;
  }

  return (
    <div className="w-full overflow-x-auto p-px">
      <Table>
        <TableHeader className="bg-muted">
          <TableRow className="h-8">
            <TableHead className="w-[4%] py-1.5 text-xs font-semibold text-foreground/90 text-center">
              No
            </TableHead>
            <TableHead className="w-[20%] py-1.5 text-xs font-semibold text-foreground/90">
              Step Details
            </TableHead>
            <TableHead className="w-[22%] py-1.5 text-xs font-semibold text-foreground/90">
              Execution
            </TableHead>
            <TableHead className="w-[18%] py-1.5 text-xs font-semibold text-foreground/90">
              Dependencies
            </TableHead>
            <TableHead className="w-[18%] py-1.5 text-xs font-semibold text-foreground/90">
              Configuration
            </TableHead>
            <TableHead className="w-[18%] py-1.5 text-xs font-semibold text-foreground/90">
              Conditions
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {steps.map((step, idx) => (
            <DAGStepTableRow key={idx} step={step} index={idx} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export default DAGStepTable;
