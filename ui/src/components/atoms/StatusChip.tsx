import { Chip } from '@mui/material';
import React from 'react';
import { statusColorMapping } from '../../consts';
import { SchedulerStatus } from '../../models';

type Props = {
  status?: SchedulerStatus;
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
