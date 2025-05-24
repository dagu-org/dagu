import { Calendar, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
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
    <div className="bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 rounded-2xl p-6 mb-6 border border-slate-200 dark:border-slate-700 shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-slate-600 dark:text-slate-400 mb-2">
            {workflowToDisplay.rootWorkflowId !== workflowToDisplay.workflowId && (
              <>
                <a
                  href={`/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`}
                  onClick={handleRootWorkflowClick}
                  className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                >
                  {workflowToDisplay.rootWorkflowName}
                </a>
                <span className="text-slate-400 dark:text-slate-500 mx-1">/</span>
              </>
            )}
            
            {workflowToDisplay.parentWorkflowName &&
              workflowToDisplay.parentWorkflowId &&
              workflowToDisplay.parentWorkflowName !== workflowToDisplay.rootWorkflowName &&
              workflowToDisplay.parentWorkflowName !== workflowToDisplay.name && (
                <>
                  <a
                    href={`/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&childWorkflowId=${workflowToDisplay.parentWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`}
                    onClick={handleParentWorkflowClick}
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                  >
                    {workflowToDisplay.parentWorkflowName}
                  </a>
                  <span className="text-slate-400 dark:text-slate-500 mx-1">/</span>
                </>
              )}
          </nav>
          
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 truncate">
            {workflowToDisplay.name}
          </h1>
        </div>
        
        {/* Actions */}
        {workflowToDisplay.workflowId === workflowToDisplay.rootWorkflowId && (
          <div className="flex-shrink-0">
            <DAGActions
              status={workflowToDisplay}
              dag={dag}
              fileName={fileName}
              refresh={refreshFn}
              displayMode="compact"
              navigateToStatusTab={navigateToStatusTab}
            />
          </div>
        )}
      </div>

      {/* Status and metadata row */}
      {workflowToDisplay.status != Status.NotStarted && (
        <div className="flex flex-wrap items-center gap-4 lg:gap-6">
          {/* Status */}
          {workflowToDisplay.status && (
            <div className="flex items-center gap-2">
              <StatusChip status={workflowToDisplay.status} size="md">
                {workflowToDisplay.statusLabel || ''}
              </StatusChip>
            </div>
          )}
          
          {/* Metadata items */}
          <div className="flex flex-wrap items-center gap-4 lg:gap-6 text-sm">
            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
              <Calendar className="h-4 w-4 text-slate-500" />
              <div className="flex flex-col">
                <span className="font-medium">
                  {workflowToDisplay?.startedAt
                    ? dayjs(workflowToDisplay.startedAt).format('MMM D, HH:mm:ss')
                    : '--'}
                </span>
                {workflowToDisplay?.startedAt && (
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    {dayjs(workflowToDisplay.startedAt).format('z')}
                  </span>
                )}
              </div>
            </div>
            
            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
              <Timer className="h-4 w-4 text-slate-500" />
              <span className="font-medium">
                {workflowToDisplay.finishedAt
                  ? formatDuration(workflowToDisplay.startedAt, workflowToDisplay.finishedAt)
                  : workflowToDisplay.startedAt
                    ? formatDuration(workflowToDisplay.startedAt, dayjs().toISOString())
                    : '--'}
              </span>
            </div>
            
            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 ml-auto">
              <span className="font-medium text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">Workflow ID</span>
              <code className="bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 px-3 py-1.5 rounded-md text-xs font-mono border">
                {workflowToDisplay.rootWorkflowId}
              </code>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGHeader;
