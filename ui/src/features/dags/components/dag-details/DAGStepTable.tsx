/**
 * DAGStepTable component displays a table of steps in a DAG.
 *
 * @module features/dags/components/dag-details
 */
import React, { CSSProperties } from 'react';
import DAGStepTableRow from './DAGStepTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../../../../ui/BorderedBox';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the DAGStepTable component
 */
type Props = {
  /** List of steps to display */
  steps: components['schemas']['Step'][];
};

/**
 * Column width styles for the table
 */
const StepConfigTabColStyles = [
  { maxWidth: '200px' },
  { maxWidth: '200px' },
  { maxWidth: '300px' },
  { maxWidth: '220px' },
  { maxWidth: '150px' },
  { maxWidth: '80px' },
  {},
];

/**
 * DAGStepTable displays a table of steps in a DAG with their properties
 */
function DAGStepTable({ steps }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: 'fixed',
    wordWrap: 'break-word',
  };

  const styles = StepConfigTabColStyles;
  let i = 0;

  // Don't render if there are no steps
  if (!steps.length) {
    return null;
  }

  return (
    <BorderedBox>
      <Table size="small" sx={tableStyle}>
        <TableHead>
          <TableRow>
            <TableCell style={styles[i++]}>Name</TableCell>
            <TableCell style={styles[i++]}>Description</TableCell>
            <TableCell style={styles[i++]}>Command</TableCell>
            <TableCell style={styles[i++]}>Args</TableCell>
            <TableCell style={styles[i++]}>Dir</TableCell>
            <TableCell style={styles[i++]}>Repeat</TableCell>
            <TableCell style={styles[i++]}>Preconditions</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {steps.map((step, idx) => (
            <DAGStepTableRow key={idx} step={step} />
          ))}
        </TableBody>
      </Table>
    </BorderedBox>
  );
}

export default DAGStepTable;
