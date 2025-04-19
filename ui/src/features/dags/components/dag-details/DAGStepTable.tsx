/**
 * DAGStepTable component displays a table of steps in a DAG.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import DAGStepTableRow from './DAGStepTableRow';
import { components } from '../../../../api/v2/schema';
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../../components/ui/table';
import { cn } from '../../../../lib/utils';

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
    <div className="mb-6 border border-slate-200 dark:border-slate-700 rounded-md overflow-hidden bg-white dark:bg-slate-900">
      <div className="w-full overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[5%]">No</TableHead>
              <TableHead className="w-[20%]">Step Details</TableHead>
              <TableHead className="w-[25%]">Execution</TableHead>
              <TableHead className="w-[15%]">Dependencies</TableHead>
              <TableHead className="w-[15%]">Configuration</TableHead>
              <TableHead className="w-[20%]">Conditions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {steps.map((step, idx) => (
              <DAGStepTableRow key={idx} step={step} index={idx} />
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

export default DAGStepTable;
