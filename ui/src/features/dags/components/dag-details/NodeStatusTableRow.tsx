/**
 * NodeStatusTableRow component renders a single row in the node status table.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import MultilineText from '../../../../ui/MultilineText';
import { TableCell } from '@mui/material';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { OpenInNew } from '@mui/icons-material';
import { Link } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import { NodeStatusChip } from '../common';

/**
 * Props for the NodeStatusTableRow component
 */
type Props = {
  /** Row number for display */
  rownum: number;
  /** Node data to display */
  node: components['schemas']['Node'];
  /** Request ID for log linking */
  requestId?: string;
  /** DAG name/fileId */
  name: string;
};

/**
 * NodeStatusTableRow displays information about a single node's execution status
 */
function NodeStatusTableRow({ name, rownum, node, requestId }: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  searchParams.set('remoteNode', 'local');
  if (node.step) {
    searchParams.set('step', node.step.name);
  }
  if (requestId) {
    searchParams.set('requestId', requestId);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;

  // Extract arguments for display
  let args = '';
  if (node.step.args) {
    // Use uninterpolated args to avoid render issues with very long params
    args =
      node.step.cmdWithArgs?.replace(node.step.command || '', '').trimStart() ||
      '';
  }

  return (
    <StyledTableRow>
      <TableCell> {rownum} </TableCell>
      <TableCell> {node.step.name} </TableCell>
      <TableCell>
        <MultilineText>{node.step.description}</MultilineText>
      </TableCell>
      <TableCell> {node.step.command} </TableCell>
      <TableCell> {args} </TableCell>
      <TableCell> {node.startedAt} </TableCell>
      <TableCell> {node.finishedAt} </TableCell>
      <TableCell>
        <NodeStatusChip status={node.status}>{node.statusText}</NodeStatusChip>
      </TableCell>
      <TableCell> {node.error} </TableCell>
      <TableCell>
        {node.log ? (
          <Link to={url}>
            <OpenInNew />
          </Link>
        ) : null}
      </TableCell>
    </StyledTableRow>
  );
}

export default NodeStatusTableRow;
