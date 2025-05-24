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
    <div className="bg-gradient-to-r from-gray-50 to-gray-100 rounded-lg p-4 mb-4 border border-gray-200">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-4 mb-3">
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-1 text-sm text-gray-600 mb-1">
            {/* Breadcrumb navigation */}
            {workflowToDisplay.rootWorkflowId !== workflowToDisplay.workflowId && (
              <>
                <a
                  href={`/dags/${fileName}?workflowId=${workflowToDisplay.rootWorkflowId}&workflowName=${encodeURIComponent(workflowToDisplay.rootWorkflowName)}`}
                  onClick={handleRootWorkflowClick}
                  className="text-blue-600 hover:text-blue-700 hover:underline transition-colors"
                >
                  {workflowToDisplay.rootWorkflowName}
                </a>
                <span className="text-gray-400 mx-1">/</span>
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
                    className="text-blue-600 hover:text-blue-700 hover:underline transition-colors"
                  >
                    {workflowToDisplay.parentWorkflowName}
                  </a>
                  <span className="text-gray-400 mx-1">/</span>
                </>
              )}
          </div>
          
          <h1 className="text-xl font-semibold text-gray-900 truncate">
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
              displayMode="full"
              navigateToStatusTab={navigateToStatusTab}
            />
          </div>
        )}
      </div>

      {/* Status and metadata row */}
      {workflowToDisplay.status != Status.NotStarted && (
        <div className="flex flex-wrap items-center gap-4 text-sm">
          {/* Status */}
          {workflowToDisplay.status && (
            <StatusChip status={workflowToDisplay.status} size="sm">
              {workflowToDisplay.statusLabel || ''}
            </StatusChip>
          )}
          
          {/* Metadata items */}
          <div className="flex items-center text-gray-600">
            <Calendar className="h-4 w-4 mr-1.5" />
            <span>
              {workflowToDisplay?.startedAt
                ? dayjs(workflowToDisplay.startedAt).format('MMM D, HH:mm:ss')
                : '--'}
            </span>
          </div>
          
          <div className="flex items-center text-gray-600">
            <Timer className="h-4 w-4 mr-1.5" />
            <span>
              {workflowToDisplay.finishedAt
                ? formatDuration(workflowToDisplay.startedAt, workflowToDisplay.finishedAt)
                : workflowToDisplay.startedAt
                  ? formatDuration(workflowToDisplay.startedAt, dayjs().toISOString())
                  : '--'}
            </span>
          </div>
          
          <div className="flex items-center text-gray-600 ml-auto">
            <span className="font-medium mr-1">ID:</span>
            <code className="bg-gray-200 px-2 py-1 rounded text-xs font-mono">
              {workflowToDisplay.rootWorkflowId}
            </code>
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGHeader;
