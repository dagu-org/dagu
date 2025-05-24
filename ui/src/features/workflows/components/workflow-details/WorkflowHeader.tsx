import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
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
    <div className="bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 rounded-2xl p-6 mb-6 border border-slate-200 dark:border-slate-700 shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-slate-600 dark:text-slate-400 mb-2">
            {workflow.rootWorkflowId !== workflow.workflowId && (
              <>
                <a
                  href={`/workflows/${workflow.rootWorkflowName}/${workflow.rootWorkflowId}`}
                  onClick={handleRootWorkflowClick}
                  className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                >
                  {workflow.rootWorkflowName}
                </a>
                <span className="text-slate-400 dark:text-slate-500 mx-1">
                  /
                </span>
              </>
            )}

            {workflow.parentWorkflowName &&
              workflow.parentWorkflowId &&
              workflow.parentWorkflowName !== workflow.rootWorkflowName &&
              workflow.parentWorkflowName !== workflow.name && (
                <>
                  <a
                    href="#"
                    onClick={handleParentWorkflowClick}
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                  >
                    {workflow.parentWorkflowName}
                  </a>
                  <span className="text-slate-400 dark:text-slate-500 mx-1">
                    /
                  </span>
                </>
              )}
          </nav>

          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 truncate">
            {workflow.name}
          </h1>
        </div>
      </div>

      {/* Status and metadata row */}
      {workflow.status != Status.NotStarted && (
        <div className="flex flex-wrap items-center gap-2 lg:gap-6">
          {/* Status and actions */}
          <div className="flex items-center gap-3">
            {workflow.status && (
              <StatusChip status={workflow.status} size="md">
                {workflow.statusLabel || ''}
              </StatusChip>
            )}
            <WorkflowActions
              workflow={workflow}
              name={workflow.name}
              refresh={refreshFn}
              displayMode="compact"
              isRootLevel={workflow.rootWorkflowId === workflow.workflowId}
            />
          </div>

          {/* Metadata items */}
          <div className="flex flex-wrap items-center gap-4 lg:gap-6 text-sm">
            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
              <Calendar className="h-4 w-4 text-slate-500" />
              <div className="flex flex-col">
                <span className="font-medium">
                  {workflow?.startedAt
                    ? dayjs(workflow.startedAt).format('MMM D, HH:mm:ss')
                    : '--'}
                </span>
                {workflow?.startedAt && (
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    {dayjs(workflow.startedAt).format('z')}
                  </span>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
              <Timer className="h-4 w-4 text-slate-500" />
              <span className="font-medium">
                {workflow.finishedAt
                  ? formatDuration(workflow.startedAt, workflow.finishedAt)
                  : workflow.startedAt
                    ? formatDuration(workflow.startedAt, dayjs().toISOString())
                    : '--'}
              </span>
            </div>

            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 ml-auto">
              <span className="font-medium text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Workflow ID
              </span>
              <code className="bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 px-3 py-1.5 rounded-md text-xs font-mono border">
                {workflow.workflowId}
              </code>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default WorkflowHeader;
