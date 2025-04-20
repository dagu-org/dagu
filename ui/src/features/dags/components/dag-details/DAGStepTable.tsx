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
function DAGStepTable({ steps, title }: Props) {
  // Don't render if there are no steps
  if (!steps.length) {
    return null;
  }

  return (
    <div className="w-full overflow-x-auto">
      <Table>
        <TableHeader className="bg-slate-50 dark:bg-slate-800">
          <TableRow className="border-b border-slate-200 dark:border-slate-700">
            <TableHead className="w-[4%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              No
            </TableHead>
            <TableHead className="w-[20%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Step Details
            </TableHead>
            <TableHead className="w-[22%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Execution
            </TableHead>
            <TableHead className="w-[18%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Dependencies
            </TableHead>
            <TableHead className="w-[18%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Configuration
            </TableHead>
            <TableHead className="w-[18%] py-3 text-sm font-semibold text-slate-700 dark:text-slate-300">
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
