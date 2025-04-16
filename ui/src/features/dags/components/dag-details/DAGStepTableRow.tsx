/**
 * DAGStepTableRow component renders a single row in the DAG step table.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import { TableCell } from '@mui/material';
import StyledTableRow from '../../../../ui/StyledTableRow';
import MultilineText from '../../../../ui/MultilineText';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the DAGStepTableRow component
 */
type Props = {
  /** Step definition to display */
  step: components['schemas']['Step'];
};

/**
 * DAGStepTableRow displays information about a single step in a DAG
 */
function DAGStepTableRow({ step }: Props) {
  // Format preconditions as a list
  const preconditions = step.preconditions?.map((c, index) => (
    <li key={index}>
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
