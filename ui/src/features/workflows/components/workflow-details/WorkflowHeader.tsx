import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Title from '../../../../ui/Title';
import { WorkflowActions } from '../common';

interface WorkflowHeaderProps {
  workflow: components['schemas']['WorkflowDetails'];
  refreshFn: () => void;
}

const WorkflowHeader: React.FC<WorkflowHeaderProps> = ({
  workflow,
  refreshFn,
}) => {
  const navigate = useNavigate();

  // Format duration utility function
  const formatDuration = (startDate: string, endDate: string): string => {
    if (!startDate || !endDate) return '--';

    const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
  };

  const handleRootWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/workflows/${workflow.rootWorkflowName}/${workflow.rootWorkflowId}`
    );
  };

  const handleParentWorkflowClick = (e: React.MouseEvent) => {
    e.preventDefault();
    if (workflow.parentWorkflowId) {
      const searchParams = new URLSearchParams();
      searchParams.set('childWorkflowId', workflow.workflowId);
      searchParams.set('workflowId', workflow.rootWorkflowId);
      searchParams.set('workflowName', workflow.rootWorkflowName);
      navigate(
        `/workflows/${workflow.parentWorkflowName}?${searchParams.toString()}`
      );
    }
  };

  return (
    <>
      <div className="flex flex-col gap-2">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-2 md:gap-0">
          <Title className="flex flex-wrap items-center">
            {/* Root workflow */}
            {workflow.rootWorkflowId !== workflow.workflowId && (
              <>
                <span className="text-blue-600 hover:underline font-normal">
                  <a
                    href={`/workflows/${workflow.rootWorkflowName}/${workflow.rootWorkflowId}`}
                    onClick={handleRootWorkflowClick}
                  >
                    {workflow.rootWorkflowName}
                  </a>
                </span>
                <span className="mx-2 text-slate-400">/</span>
              </>
            )}

            {/* Parent workflow (if exists and different from root and current) */}
            {workflow.parentWorkflowName &&
              workflow.parentWorkflowId &&
              workflow.parentWorkflowName !== workflow.rootWorkflowName &&
              workflow.parentWorkflowName !== workflow.name && (
                <>
                  <span className="text-blue-600 hover:underline font-normal">
                    <a href="#" onClick={handleParentWorkflowClick}>
                      {workflow.parentWorkflowName}
                    </a>
                  </span>
                  <span className="mx-2 text-slate-400">/</span>
                </>
              )}

            {/* Current workflow */}
            <span className="break-all">{workflow.name}</span>
          </Title>
        </div>
      </div>
      {workflow.status != Status.NotStarted ? (
        <div className="mt-4 mb-4">
          {/* Status chip and actions */}
          {workflow.status ? (
            <div className="mb-4 flex items-center gap-2">
              <StatusChip status={workflow.status}>
                {workflow.statusLabel || ''}
              </StatusChip>
              <WorkflowActions
                workflow={workflow}
                name={workflow.name}
                refresh={refreshFn}
                displayMode="compact"
              />
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
                  {workflow?.startedAt
                    ? dayjs(workflow.startedAt).format('YYYY-MM-DD HH:mm:ss Z')
                    : '--'}
                </span>
              </div>

              {/* Duration with icon */}
              <div className="flex items-center text-slate-600 dark:text-slate-400">
                <Timer className="mr-1.5 h-4 w-4 flex-shrink-0" />
                <span className="text-sm">
                  {workflow.finishedAt
                    ? formatDuration(workflow.startedAt, workflow.finishedAt)
                    : workflow.startedAt
                      ? formatDuration(
                          workflow.startedAt,
                          dayjs().toISOString()
                        )
                      : '--'}
                </span>
              </div>
            </div>

            {/* Right side - Workflow ID */}
            <div className="text-sm text-slate-600 dark:text-slate-400 break-all mt-3 md:mt-0">
              <span className="font-medium">Workflow ID:</span>{' '}
              {workflow.workflowId}
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
};

export default WorkflowHeader;
