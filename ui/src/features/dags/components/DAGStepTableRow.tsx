import React from 'react';
import { TableCell } from '@mui/material';
import StyledTableRow from '../../../ui/StyledTableRow';
import MultilineText from '../../../ui/MultilineText';
import { components } from '../../../api/v2/schema';

type Props = {
  step: components['schemas']['Step'];
};

function DAGStepTableRow({ step }: Props) {
  const preconditions = step.preconditions?.map((c) => (
    <li>
      {c.condition}
      {' => '}
      {c.expected}
    </li>
  ));
  return (
    <StyledTableRow>
      <TableCell className="has-text-weight-semibold"> {step.name} </TableCell>
      <TableCell>
        <MultilineText>{step.description}</MultilineText>
      </TableCell>
      <TableCell> {step.command} </TableCell>
      <TableCell> {step.args ? step.args.join(' ') : ''} </TableCell>
      <TableCell> {step.dir} </TableCell>
      <TableCell> {step.repeatPolicy?.repeat ? 'Repeat' : '-'} </TableCell>
      <TableCell>
        <ul> {preconditions} </ul>
      </TableCell>
    </StyledTableRow>
  );
}

export default DAGStepTableRow;
