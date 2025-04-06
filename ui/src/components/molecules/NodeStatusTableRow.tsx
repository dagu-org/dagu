import React from 'react';
import { Node, Step } from '../../models';
import MultilineText from '../atoms/MultilineText';
import NodeStatusChip from '../molecules/NodeStatusChip';
import { TableCell } from '@mui/material';
import StyledTableRow from '../atoms/StyledTableRow';
import { OpenInNew } from '@mui/icons-material';
import { Link } from 'react-router-dom';

type Props = {
  rownum: number;
  node: Node;
  file: string;
  name: string;
  onRequireModal: (step: Step) => void;
};

function NodeStatusTableRow({
  name,
  rownum,
  node,
  file,
  onRequireModal,
}: Props) {
  const searchParams = new URLSearchParams();
  searchParams.set('remoteNode', 'local');
  if (node.Step) {
    searchParams.set('step', node.Step.Name);
  }
  if (file) {
    searchParams.set('file', file);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;
  const buttonStyle = {
    margin: '0px',
    padding: '0px',
    border: '0px',
    background: 'none',
    outline: 'none',
  };
  let args = '';
  if (node.Step.Args) {
    // Use uninterpolated args to avoid render issues with very long params
    args =
      node.Step.CmdWithArgs?.replace(node.Step.Command || '', '').trimStart() ||
      '';
  }
  return (
    <StyledTableRow>
      <TableCell> {rownum} </TableCell>
      <TableCell> {node.Step.Name} </TableCell>
      <TableCell>
        <MultilineText>{node.Step.Description}</MultilineText>
      </TableCell>
      <TableCell> {node.Step.Command} </TableCell>
      <TableCell> {args} </TableCell>
      <TableCell> {node.StartedAt} </TableCell>
      <TableCell> {node.FinishedAt} </TableCell>
      <TableCell>
        <button style={buttonStyle} onClick={() => onRequireModal(node.Step)}>
          <NodeStatusChip status={node.Status}>
            {node.StatusText}
          </NodeStatusChip>
        </button>
      </TableCell>
      <TableCell> {node.Error} </TableCell>
      <TableCell>
        {node.Log ? (
          <Link to={url}>
            <OpenInNew />
          </Link>
        ) : null}
      </TableCell>
    </StyledTableRow>
  );
}
export default NodeStatusTableRow;
