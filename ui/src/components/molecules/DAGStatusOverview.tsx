import React from 'react';
import { Status } from '../../models';
import StatusChip from '../atoms/StatusChip';
import { Stack } from '@mui/material';
import LabeledItem from '../atoms/LabeledItem';
import { Link } from 'react-router-dom';

type Props = {
  status?: Status;
  name: string;
  file?: string;
};

function DAGStatusOverview({ status, name, file = '' }: Props) {
  const searchParams = new URLSearchParams();
  if (file) {
    searchParams.set('file', file);
  }
  const url = `/dags/${name}/scheduler-log?${searchParams.toString()}`;
  if (!status) {
    return null;
  }
  return (
    <Stack direction="column" spacing={1}>
      <LabeledItem label="Status">
        <StatusChip status={status.Status}>{status.StatusText}</StatusChip>
      </LabeledItem>
      <LabeledItem label="Request ID">{status.RequestId}</LabeledItem>
      <Stack direction="row" sx={{ alignItems: 'center' }} spacing={2}>
        <LabeledItem label="Started At">{status.StartedAt}</LabeledItem>
        <LabeledItem label="Finished At">{status.FinishedAt}</LabeledItem>
      </Stack>
      <LabeledItem label="Params">{status.Params}</LabeledItem>
      <LabeledItem label="Scheduler Log">
        <Link to={url}>{status.Log}</Link>
      </LabeledItem>
    </Stack>
  );
}
export default DAGStatusOverview;
