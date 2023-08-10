import React, { CSSProperties } from 'react';
import { Step } from '../../models';
import DAGStepTableRow from './DAGStepTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../atoms/BorderedBox';

type Props = {
  steps: Step[];
};

function DAGStepTable({ steps }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: 'fixed',
    wordWrap: 'break-word',
  };
  const styles = StepConfigTabColStyles;
  let i = 0;
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
            <DAGStepTableRow key={idx} step={step}></DAGStepTableRow>
          ))}
        </TableBody>
      </Table>
    </BorderedBox>
  );
}
export default DAGStepTable;

const StepConfigTabColStyles = [
  { maxWidth: '200px' },
  { maxWidth: '200px' },
  { maxWidth: '300px' },
  { maxWidth: '220px' },
  { maxWidth: '150px' },
  { maxWidth: '80px' },
  {},
];
