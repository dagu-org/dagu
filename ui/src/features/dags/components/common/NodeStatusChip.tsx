/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import React from 'react';
import { Chip } from '@mui/material';
import { nodeStatusColorMapping } from '../../../../consts';
import { NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the NodeStatusChip component
 */
type Props = {
  /** Status code of the node */
  status: NodeStatus;
  /** Text to display in the chip */
  children: string;
};

/**
 * NodeStatusChip displays a colored chip based on the node status
 * Uses the nodeStatusColorMapping to determine the appropriate styling
 */
function NodeStatusChip({ status, children }: Props) {
  const style = nodeStatusColorMapping[status] || {};
  return <Chip sx={style} label={children} />;
}

export default NodeStatusChip;
