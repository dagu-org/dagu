import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Title from '../../../../ui/Title';
import { RootWorkflowContext } from '../../contexts/RootWorkflowContext';
import { DAGActions } from '../common';

interface DAGHeaderProps {
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  currentWorkflow: components['schemas']['WorkflowDetails'];
  fileName: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  navigateToStatusTab?: () => void;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  currentWorkflow,
  fileName,
  refreshFn,
  formatDuration,
  navigateToStatusTab,
}) => (
  <>
    <div className="flex flex-row items-center justify-between">
      <Title>{dag?.name}</Title>
      <RootWorkflowContext.Consumer>
        {(status) =>
          status.data ? (
            <DAGActions
              status={status.data}
              dag={dag}
              fileName={fileName}
              refresh={refreshFn}
              displayMode="full"
              navigateToStatusTab={navigateToStatusTab}
            />
          ) : null
        }
      </RootWorkflowContext.Consumer>
    </div>
    {currentWorkflow.status != Status.NotStarted ? (
      <div className="flex flex-row items-center gap-4">
        {currentWorkflow.status ? (
          <StatusChip status={currentWorkflow.status}>
            {currentWorkflow.statusLabel || ''}
          </StatusChip>
        ) : null}

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Calendar className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {currentWorkflow?.startedAt
              ? dayjs(currentWorkflow.startedAt).format('YYYY-MM-DD HH:mm:ss Z')
              : '--'}
          </span>
        </div>

        <div className="flex flex-row items-center text-slate-600 dark:text-slate-400">
          <Timer className="mr-1.5 h-4 w-4" />
          <span className="text-sm">
            {currentWorkflow.finishedAt
              ? formatDuration(
                  currentWorkflow.startedAt,
                  currentWorkflow.finishedAt
                )
              : currentWorkflow.startedAt
                ? formatDuration(
                    currentWorkflow.startedAt,
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
