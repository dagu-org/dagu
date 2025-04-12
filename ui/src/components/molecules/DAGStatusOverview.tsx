import React from 'react';
import StatusChip from '../atoms/StatusChip';
import { Stack } from '@mui/material';
import LabeledItem from '../atoms/LabeledItem';
import { Link } from 'react-router-dom';
import { components } from '../../api/v2/schema';

type Props = {
  status?: components['schemas']['RunDetails'];
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
        <StatusChip status={status.status}>{status.statusText}</StatusChip>
      </LabeledItem>
      <LabeledItem label="Request ID">{status.requestId}</LabeledItem>
      <Stack direction="row" sx={{ alignItems: 'center' }} spacing={2}>
        <LabeledItem label="Started At">{status.startedAt}</LabeledItem>
        <LabeledItem label="Finished At">{status.finishedAt}</LabeledItem>
      </Stack>
      <LabeledItem label="Params">{status.params}</LabeledItem>
      <LabeledItem label="Scheduler Log">
        <Link to={url}>{status.log}</Link>
      </LabeledItem>
    </Stack>
  );
}
export default DAGStatusOverview;
