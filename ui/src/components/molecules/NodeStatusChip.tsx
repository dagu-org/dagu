import React from 'react';
import { Chip } from '@mui/material';
import { nodeStatusColorMapping } from '../../consts';
import { NodeStatus } from '../../api/v2/schema';

type Props = {
  status: NodeStatus;
  children: string;
};

function NodeStatusChip({ status, children }: Props) {
  const style = nodeStatusColorMapping[status] || {};
  return <Chip sx={style} label={children} />;
}

export default NodeStatusChip;
