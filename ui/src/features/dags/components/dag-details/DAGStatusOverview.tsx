/**
 * DAGStatusOverview component displays summary information about a DAG run.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import StatusChip from '../../../../ui/StatusChip';
import { Stack } from '@mui/material';
import LabeledItem from '../../../../ui/LabeledItem';
import { Link } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the DAGStatusOverview component
 */
type Props = {
  /** DAG run details */
  status?: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileId: string;
  /** Request ID of the execution */
  requestId?: string;
};

/**
 * DAGStatusOverview displays summary information about a DAG run
 * including status, request ID, timestamps, and parameters
 */
function DAGStatusOverview({ status, fileId, requestId = '' }: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  if (requestId) {
    searchParams.set('requestId', requestId);
  }
  const url = `/dags/${fileId}/scheduler-log?${searchParams.toString()}`;

  // Don't render if no status is provided
  if (!status) {
    return null;
  }

  return (
    <Stack direction="column" spacing={1}>
      <LabeledItem label="Status">
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <StatusChip status={status.status}>{status.statusText}</StatusChip>
          <Link to={url}>View Log</Link>
        </Stack>
      </LabeledItem>
      <LabeledItem label="Request ID">{status.requestId}</LabeledItem>
      <Stack direction="row" sx={{ alignItems: 'center' }} spacing={2}>
        <LabeledItem label="Started At">{status.startedAt}</LabeledItem>
        <LabeledItem label="Finished At">{status.finishedAt}</LabeledItem>
      </Stack>
      <LabeledItem label="Params">{status.params}</LabeledItem>
    </Stack>
  );
}

export default DAGStatusOverview;
