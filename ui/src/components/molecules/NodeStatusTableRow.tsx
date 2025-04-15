import React from 'react';
import MultilineText from '../atoms/MultilineText';
import NodeStatusChip from '../molecules/NodeStatusChip';
import { TableCell } from '@mui/material';
import StyledTableRow from '../atoms/StyledTableRow';
import { OpenInNew } from '@mui/icons-material';
import { Link } from 'react-router-dom';
import { components } from '../../api/v2/schema';

type Props = {
  rownum: number;
  node: components['schemas']['Node'];
  requestId?: string;
  name: string;
};

function NodeStatusTableRow({ name, rownum, node, requestId }: Props) {
  const searchParams = new URLSearchParams();
  searchParams.set('remoteNode', 'local');
  if (node.step) {
    searchParams.set('step', node.step.name);
  }
  if (requestId) {
    searchParams.set('requestId', requestId);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;
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
