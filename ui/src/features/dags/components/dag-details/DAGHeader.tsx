import React from 'react';
import Title from '../../../../ui/Title';
import { DAGActions } from '../common';
import StatusChip from '../../../../ui/StatusChip';
import { Calendar, Timer } from 'lucide-react';
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
    <div className="flex flex-row items-center justify-between">
      <Title>{dag?.name}</Title>
      <RunDetailsContext.Consumer>
        {(status) =>
          status.data ? (
            <DAGActions
              status={status.data}
              dag={dag}
              fileId={fileId}
              refresh={refreshFn}
              displayMode="full"
            />
          ) : null
        }
      </RunDetailsContext.Consumer>
    </div>
    {latestRun.status != Status.NotStarted ? (
      <div className="flex flex-row items-center gap-4">
        {latestRun.status ? (
          <StatusChip status={latestRun.status}>
            {latestRun.statusText || ''}
          </StatusChip>
        ) : null}

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Calendar className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {latestRun?.finishedAt
              ? moment(latestRun.finishedAt).format('MMM D, YYYY HH:mm:ss Z')
              : '--'}
          </span>
        </div>

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Timer className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {latestRun.finishedAt
              ? formatDuration(latestRun.startedAt, latestRun.finishedAt)
              : latestRun.startedAt
                ? formatDuration(latestRun.startedAt, moment().toISOString())
                : '--'}
          </span>
        </div>
      </div>
    ) : null}
  </>
);

export default DAGHeader;
