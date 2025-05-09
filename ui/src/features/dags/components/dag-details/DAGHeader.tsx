import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Title from '../../../../ui/Title';
import { WorkflowDetailsContext } from '../../contexts/DAGStatusContext';
import { DAGActions } from '../common';

interface DAGHeaderProps {
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  latestWorkflow: components['schemas']['WorkflowDetails'];
  fileName: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  latestWorkflow,
  fileName,
  refreshFn,
  formatDuration,
}) => (
  <>
    <div className="flex flex-row items-center justify-between">
      <Title>{dag?.name}</Title>
      <WorkflowDetailsContext.Consumer>
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
      </WorkflowDetailsContext.Consumer>
    </div>
    {latestWorkflow.status != Status.NotStarted ? (
      <div className="flex flex-row items-center gap-4">
        {latestWorkflow.status ? (
          <StatusChip status={latestWorkflow.status}>
            {latestWorkflow.statusLabel || ''}
          </StatusChip>
        ) : null}

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Calendar className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {latestWorkflow?.startedAt
              ? dayjs(latestWorkflow.startedAt).format('YYYY-MM-DD HH:mm:ss Z')
              : '--'}
          </span>
        </div>

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Timer className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {latestWorkflow.finishedAt
              ? formatDuration(
                  latestWorkflow.startedAt,
                  latestWorkflow.finishedAt
                )
              : latestWorkflow.startedAt
                ? formatDuration(
                    latestWorkflow.startedAt,
                    dayjs().toISOString()
                  )
                : '--'}
          </span>
        </div>
      </div>
    ) : null}
  </>
);

export default DAGHeader;
