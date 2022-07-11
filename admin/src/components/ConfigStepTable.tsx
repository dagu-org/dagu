import React, { CSSProperties } from 'react';
import { Step } from '../models/Step';
import ConfigStepTableRow from './ConfigStepTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from './BorderedBox';

type Props = {
  steps: Step[];
};

function ConfigStepTable({ steps }: Props) {
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
            <ConfigStepTableRow key={idx} step={step}></ConfigStepTableRow>
          ))}
        </TableBody>
      </Table>
    </BorderedBox>
  );
}
export default ConfigStepTable;

const StepConfigTabColStyles = [
  { maxWidth: '200px' },
  { maxWidth: '200px' },
  { maxWidth: '300px' },
  { maxWidth: '220px' },
  { maxWidth: '150px' },
  { maxWidth: '80px' },
  {},
];
