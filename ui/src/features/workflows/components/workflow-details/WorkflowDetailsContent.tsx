import React from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGStatus } from '../../../../features/dags/components';
import Title from '../../../../ui/Title';
import { WorkflowContext } from '../../contexts/WorkflowContext';

type WorkflowDetailsContentProps = {
  name: string;
  workflow: components['schemas']['WorkflowDetails'];
  refreshFn: () => void;
  workflowId?: string;
};

const WorkflowDetailsContent: React.FC<WorkflowDetailsContentProps> = ({
  name,
  workflow,
  refreshFn,
  workflowId = 'latest',
}) => {
  return (
    <WorkflowContext.Provider
      value={{
        refresh: refreshFn,
        name: name || '',
        workflowId: workflowId || '',
      }}
    >
      <div className="w-full flex flex-col">
        <div className="mb-6">
          <Title>{workflow.name}</Title>
          <p className="text-sm text-muted-foreground">
            Workflow ID:{' '}
            <span className="font-mono">{workflow.workflowId}</span>
          </p>
        </div>

        <div className="flex-1">
          <DAGStatus workflow={workflow} fileName={name || ''} />
        </div>
      </div>
    </WorkflowContext.Provider>
  );
};

export default WorkflowDetailsContent;
