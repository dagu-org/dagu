import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
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
}) => {
  const navigate = useNavigate();

  const handleRootWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?workflowId=${currentWorkflow.rootWorkflowId}&workflowName=${encodeURIComponent(currentWorkflow.rootWorkflowName)}`
    );
  };

  const handleParentWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?childWorkflowId=${currentWorkflow.parentWorkflowId}&workflowId=${currentWorkflow.rootWorkflowId}&workflowName=${encodeURIComponent(currentWorkflow.rootWorkflowName)}`
    );
  };

  return (
    <>
      <div className="flex flex-col gap-2">
        <div className="flex flex-row items-center justify-between">
          <Title className="flex items-center">
            {/* Root workflow */}
            {currentWorkflow.rootWorkflowId !== currentWorkflow.workflowId && (
              <>
                <span className="text-blue-600 hover:underline font-normal">
                  <a
                    href={`/dags/${fileName}?workflowId=${currentWorkflow.rootWorkflowId}&workflowName=${encodeURIComponent(currentWorkflow.rootWorkflowName)}`}
                    onClick={handleRootWorkflowClick}
                  >
                    {currentWorkflow.rootWorkflowName}
                  </a>
                </span>
                <span className="mx-2 text-slate-400">/</span>
              </>
            )}

            {/* Parent workflow (if exists and different from root and current) */}
            {currentWorkflow.parentWorkflowName &&
              currentWorkflow.parentWorkflowId &&
              currentWorkflow.parentWorkflowName !==
                currentWorkflow.rootWorkflowName &&
              currentWorkflow.parentWorkflowName !== currentWorkflow.name && (
                <>
                  <span className="text-blue-600 hover:underline font-normal">
                    <a
                      href={`/dags/${fileName}?workflowId=${currentWorkflow.rootWorkflowId}&childWorkflowId=${currentWorkflow.parentWorkflowId}&workflowName=${encodeURIComponent(currentWorkflow.rootWorkflowName)}`}
                      onClick={handleParentWorkflowClick}
                    >
                      {currentWorkflow.parentWorkflowName}
                    </a>
                  </span>
                  <span className="mx-2 text-slate-400">/</span>
                </>
              )}

            {/* Current workflow */}
            <span>{currentWorkflow.name}</span>
          </Title>
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
                ? dayjs(currentWorkflow.startedAt).format(
                    'YYYY-MM-DD HH:mm:ss Z'
                  )
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
};

export default DAGHeader;
