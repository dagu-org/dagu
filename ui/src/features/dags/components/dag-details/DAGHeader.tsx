import React from 'react';
import Title from '../../../../ui/Title';
import { DAGActions } from '../common';
import StatusChip from '../../../../ui/StatusChip';
import { CalendarToday, TimerSharp } from '@mui/icons-material';
import { Stack, Box } from '@mui/material';
import moment from 'moment-timezone';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { components, Status } from '../../../../api/v2/schema';

interface DAGHeaderProps {
  dag: any;
  latestRun: components['schemas']['RunDetails'];
  fileId: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  latestRun,
  fileId,
  refreshFn,
  formatDuration,
}) => (
  <>
    <Box
      sx={{
        mx: 4,
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <Title>{dag?.name}</Title>
      <RunDetailsContext.Consumer>
        {(status) =>
          status.data ? (
            <DAGActions
              status={status.data}
              dag={dag}
              fileId={fileId}
              refresh={refreshFn}
            />
          ) : null
        }
      </RunDetailsContext.Consumer>
    </Box>
    {latestRun.status != Status.NotStarted ? (
      <Stack direction="row" spacing={2} sx={{ mx: 4, alignItems: 'center' }}>
        {latestRun.status ? (
          <StatusChip status={latestRun.status}>
            {latestRun.statusText || ''}
          </StatusChip>
        ) : null}

        <Stack
          direction="row"
          color={'text.secondary'}
          sx={{ alignItems: 'center', ml: 1 }}
        >
          <CalendarToday sx={{ mr: 0.5 }} />
          {latestRun?.finishedAt
            ? moment(latestRun.finishedAt).format('MMM D, YYYY HH:mm:ss Z')
            : '--'}
        </Stack>

        <Stack
          direction="row"
          color={'text.secondary'}
          sx={{ alignItems: 'center', ml: 1 }}
        >
          <TimerSharp sx={{ mr: 0.5 }} />
          {latestRun.finishedAt
            ? formatDuration(latestRun.startedAt, latestRun.finishedAt)
            : latestRun.startedAt
              ? formatDuration(latestRun.startedAt, moment().toISOString())
              : '--'}
        </Stack>
      </Stack>
    ) : null}
  </>
);

export default DAGHeader;
