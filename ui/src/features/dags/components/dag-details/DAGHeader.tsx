import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Title from '../../../../ui/Title';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { DAGActions } from '../common';

interface DAGHeaderProps {
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  latestRun: components['schemas']['RunDetails'];
  fileName: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  latestRun,
  fileName,
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
              fileName={fileName}
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
            {latestRun?.startedAt
              ? dayjs(latestRun.startedAt).format('YYYY-MM-DD HH:mm:ss Z')
              : '--'}
          </span>
        </div>

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Timer className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {latestRun.finishedAt
              ? formatDuration(latestRun.startedAt, latestRun.finishedAt)
              : latestRun.startedAt
                ? formatDuration(latestRun.startedAt, dayjs().toISOString())
                : '--'}
          </span>
        </div>
      </div>
    ) : null}
  </>
);

export default DAGHeader;
