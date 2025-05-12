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
  const rootWorkflowContext = React.useContext(RootWorkflowContext);

  // Use the workflow from context if available, otherwise use the prop
  const workflowToDisplay = rootWorkflowContext.data || currentWorkflow;

  const handleRootWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`
    );
  };

  const handleParentWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?childWorkflowId=${workflowToDisplay.parentWorkflowId}&workflowId=${workflowToDisplay.rootWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`
    );
  };

  return (
    <>
      <div className="flex flex-col gap-2">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-2 md:gap-0">
          <Title className="flex flex-wrap items-center">
            {/* Root workflow */}
            {workflowToDisplay.rootWorkflowId !==
              workflowToDisplay.workflowId && (
              <>
                <span className="text-blue-600 hover:underline font-normal">
                  <a
                    href={`/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`}
                    onClick={handleRootWorkflowClick}
                  >
                    {workflowToDisplay.rootWorkflowName}
                  </a>
                </span>
                <span className="mx-2 text-slate-400">/</span>
              </>
            )}

            {/* Parent workflow (if exists and different from root and current) */}
            {workflowToDisplay.parentWorkflowName &&
              workflowToDisplay.parentWorkflowId &&
              workflowToDisplay.parentWorkflowName !==
                workflowToDisplay.rootWorkflowName &&
              workflowToDisplay.parentWorkflowName !==
                workflowToDisplay.name && (
                <>
                  <span className="text-blue-600 hover:underline font-normal">
                    <a
                      href={`/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&childWorkflowId=${workflowToDisplay.parentWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`}
                      onClick={handleParentWorkflowClick}
                    >
                      {workflowToDisplay.parentWorkflowName}
                    </a>
                  </span>
                  <span className="mx-2 text-slate-400">/</span>
                </>
              )}

            {/* Current workflow */}
            <span className="break-all">{workflowToDisplay.name}</span>
          </Title>
          {/* Only show DAG actions for root workflows, not for child workflows */}
          {workflowToDisplay.workflowId ===
            workflowToDisplay.rootWorkflowId && (
            <DAGActions
              status={workflowToDisplay}
              dag={dag}
              fileName={fileName}
              refresh={refreshFn}
              displayMode="full"
              navigateToStatusTab={navigateToStatusTab}
            />
          )}
        </div>
      </div>
      {workflowToDisplay.status != Status.NotStarted ? (
        <div className="mt-4 mb-4">
          {/* Status chip */}
          {workflowToDisplay.status ? (
            <div className="mb-4">
              <StatusChip status={workflowToDisplay.status}>
                {workflowToDisplay.statusLabel || ''}
              </StatusChip>
            </div>
          ) : null}

          {/* Simple flex layout for metadata */}
          <div className="flex flex-col md:flex-row md:items-center md:justify-between">
            {/* Left side - Date and Duration in a row on desktop, column on mobile */}
            <div className="flex flex-col md:flex-row md:items-center gap-3 md:gap-6">
              {/* Date with icon */}
              <div className="flex items-center text-slate-600 dark:text-slate-400">
                <Calendar className="mr-1.5 h-4 w-4 flex-shrink-0" />
                <span className="text-sm">
                  {workflowToDisplay?.startedAt
                    ? dayjs(workflowToDisplay.startedAt).format(
                        'YYYY-MM-DD HH:mm:ss Z'
                      )
                    : '--'}
                </span>
              </div>

              {/* Duration with icon */}
              <div className="flex items-center text-slate-600 dark:text-slate-400">
                <Timer className="mr-1.5 h-4 w-4 flex-shrink-0" />
                <span className="text-sm">
                  {workflowToDisplay.finishedAt
                    ? formatDuration(
                        workflowToDisplay.startedAt,
                        workflowToDisplay.finishedAt
                      )
                    : workflowToDisplay.startedAt
                      ? formatDuration(
                          workflowToDisplay.startedAt,
                          dayjs().toISOString()
                        )
                      : '--'}
                </span>
              </div>
            </div>

            {/* Right side - Workflow ID */}
            <div className="text-sm text-slate-600 dark:text-slate-400 break-all mt-3 md:mt-0">
              <span className="font-medium">Workflow ID:</span>{' '}
              {workflowToDisplay.rootWorkflowId}
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
};

export default DAGHeader;
