import React from 'react';
import ExecutionLog from './ExecutionLog';
import LogSideModal from './LogSideModal';
import StepLog from './StepLog';

type LogViewerProps = {
  isOpen: boolean;
  onClose: () => void;
  logType: 'execution' | 'step';
  dagName: string;
  requestId: string;
  stepName?: string;
  isInModal?: boolean;
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
  requestId,
  stepName,
  isInModal = true,
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
      requestId={requestId}
      stepName={stepName}
      logType={logType}
    >
      <div className="h-full">
        {logType === 'execution' ? (
          <ExecutionLog name={dagName} requestId={requestId} />
        ) : (
          stepName && (
            <StepLog
              dagName={dagName}
              requestId={requestId}
              stepName={stepName}
            />
          )
        )}
      </div>
    </LogSideModal>
  );
};

export default LogViewer;
