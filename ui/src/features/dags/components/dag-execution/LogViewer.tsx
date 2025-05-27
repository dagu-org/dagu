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
  dagRunId: string;
  stepName?: string;
  isInModal?: boolean;
  dagRun?: components['schemas']['DAGRunDetails'];
  stream?: 'stdout' | 'stderr';
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
  dagRunId,
  stepName,
  isInModal = true,
  dagRun,
  stream = 'stdout',
}) => {
  // Determine the title based on the log type
  const title =
    logType === 'execution'
      ? `Execution Log: ${dagName}`
      : `Step Log (${stream}): ${stepName}`;

  return (
    <LogSideModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      isInModal={isInModal}
      dagName={dagName}
      dagRunId={dagRunId}
      stepName={stepName}
      logType={logType}
    >
      <div className="h-full">
        {logType === 'execution' ? (
          <ExecutionLog
            name={dagName}
            dagRunId={dagRunId}
            dagRun={dagRun}
            stream={stream}
          />
        ) : (
          stepName && (
            <StepLog
              dagName={dagName}
              dagRunId={dagRunId}
              stepName={stepName}
              dagRun={dagRun}
              stream={stream}
            />
          )
        )}
      </div>
    </LogSideModal>
  );
};

export default LogViewer;
