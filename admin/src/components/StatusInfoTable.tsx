import React from 'react';
import { Status } from '../models/Status';
import { DetailTabId } from '../models/DAGData';
import StatusChip from './StatusChip';
import { Stack } from '@mui/material';
import LabeledItem from './LabeledItem';

type Props = {
  status?: Status;
  name: string;
  file?: string;
};

function StatusInfoTable({ status, name, file = '' }: Props) {
  const url = `/dags/${name}?t=${DetailTabId.ScLog}&file=${encodeURI(file)}`;
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
        <a href={url}> {status.Log} </a>
      </LabeledItem>
    </Stack>
  );
}
export default StatusInfoTable;
