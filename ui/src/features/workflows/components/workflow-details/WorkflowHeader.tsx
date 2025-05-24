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
    <div className="bg-gradient-to-r from-gray-50 to-gray-100 rounded-lg p-4 mb-4 border border-gray-200">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-4 mb-3">
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-1 text-sm text-gray-600 mb-1">
            {/* Breadcrumb navigation */}
            {workflow.rootWorkflowId !== workflow.workflowId && (
              <>
                <a
                  href={`/workflows/${workflow.rootWorkflowName}/${workflow.rootWorkflowId}`}
                  onClick={handleRootWorkflowClick}
                  className="text-blue-600 hover:text-blue-700 hover:underline transition-colors"
                >
                  {workflow.rootWorkflowName}
                </a>
                <span className="text-gray-400 mx-1">/</span>
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
                    className="text-blue-600 hover:text-blue-700 hover:underline transition-colors"
                  >
                    {workflow.parentWorkflowName}
                  </a>
                  <span className="text-gray-400 mx-1">/</span>
                </>
              )}
          </div>
          
          <h1 className="text-xl font-semibold text-gray-900 truncate">
            {workflow.name}
          </h1>
        </div>
      </div>

      {/* Status and metadata row */}
      {workflow.status != Status.NotStarted && (
        <div className="flex flex-wrap items-center gap-4 text-sm">
          {/* Status and actions */}
          <div className="flex items-center gap-2">
            {workflow.status && (
              <StatusChip status={workflow.status} size="sm">
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
          <div className="flex items-center text-gray-600">
            <Calendar className="h-4 w-4 mr-1.5" />
            <span>
              {workflow?.startedAt
                ? dayjs(workflow.startedAt).format('MMM D, HH:mm:ss')
                : '--'}
            </span>
          </div>
          
          <div className="flex items-center text-gray-600">
            <Timer className="h-4 w-4 mr-1.5" />
            <span>
              {workflow.finishedAt
                ? formatDuration(workflow.startedAt, workflow.finishedAt)
                : workflow.startedAt
                  ? formatDuration(workflow.startedAt, dayjs().toISOString())
                  : '--'}
            </span>
          </div>
          
          <div className="flex items-center text-gray-600 ml-auto">
            <span className="font-medium mr-1">ID:</span>
            <code className="bg-gray-200 px-2 py-1 rounded text-xs font-mono">
              {workflow.workflowId}
            </code>
          </div>
        </div>
      )}
    </div>
  );
};

export default WorkflowHeader;
