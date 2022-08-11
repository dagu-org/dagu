import React from 'react';
import { Step } from '../models';
import MultilineText from './MultilineText';
import { TableCell } from '@mui/material';
import StyledTableRow from './StyledTableRow';

type Props = {
  step: Step;
};

function ConfigStepTableRow({ step }: Props) {
  const preconditions = step.Preconditions.map((c) => (
    <li>
      {c.Condition}
      {' => '}
      {c.Expected}
    </li>
  ));
  return (
    <StyledTableRow>
      <TableCell className="has-text-weight-semibold"> {step.Name} </TableCell>
      <TableCell>
        <MultilineText>{step.Description}</MultilineText>
      </TableCell>
      <TableCell> {step.Command} </TableCell>
      <TableCell> {step.Args ? step.Args.join(' ') : ''} </TableCell>
      <TableCell> {step.Dir} </TableCell>
      <TableCell> {step.RepeatPolicy.Repeat ? 'Repeat' : '-'} </TableCell>
      <TableCell>
        <ul> {preconditions} </ul>
      </TableCell>
    </StyledTableRow>
  );
}

export default ConfigStepTableRow;
