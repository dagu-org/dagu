import React from 'react';
import { components } from '../../../../api/v2/schema';
import ExecutionLog from './ExecutionLog';
import LogSideModal from './LogSideModal';
import StepLog from './StepLog';

type LogViewerProps = {
  isOpen: boolean;
  onClose: () => void;
  logType: 'execution' | 'step';
  dagName: string;
  workflowId: string;
  stepName?: string;
  isInModal?: boolean;
  workflow?: components['schemas']['WorkflowDetails'];
};

/**
 * LogViewer is a wrapper component that displays logs in a side modal
 * It can show either execution logs or step logs based on the logType prop
 */
const LogViewer: React.FC<LogViewerProps> = ({
  isOpen,
  onClose,
  logType,
  dagName,
  workflowId,
  stepName,
  isInModal = true,
  workflow,
}) => {
  // Determine the title based on the log type
  const title =
    logType === 'execution'
      ? `Execution Log: ${dagName}`
      : `Step Log: ${stepName}`;

  return (
    <LogSideModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      isInModal={isInModal}
      dagName={dagName}
      workflowId={workflowId}
      stepName={stepName}
      logType={logType}
    >
      <div className="h-full">
        {logType === 'execution' ? (
          <ExecutionLog
            name={dagName}
            workflowId={workflowId}
            workflow={workflow}
          />
        ) : (
          stepName && (
            <StepLog
              dagName={dagName}
              workflowId={workflowId}
              stepName={stepName}
              workflow={workflow}
            />
          )
        )}
      </div>
    </LogSideModal>
  );
};

export default LogViewer;
