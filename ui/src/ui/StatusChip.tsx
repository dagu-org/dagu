import { Chip } from '@mui/material';
import React from 'react';
import { statusColorMapping } from '../consts';
import { Status } from '../api/v2/schema';

type Props = {
  status?: Status;
  children: string;
};

function StatusChip({ status, children }: Props) {
  const style = () => {
    if (!status) {
      return {};
    }
    return statusColorMapping[status] || {};
  };
  return <Chip sx={style} size="small" label={children} />;
}

export default StatusChip;
