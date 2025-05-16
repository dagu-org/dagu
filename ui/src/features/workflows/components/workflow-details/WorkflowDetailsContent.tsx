import React from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGStatus } from '../../../../features/dags/components';
import { WorkflowContext } from '../../contexts/WorkflowContext';
import WorkflowHeader from './WorkflowHeader';

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
        {/* Display breadcrumbs and workflow details in the header */}
        <div className="mb-6">
          <WorkflowHeader workflow={workflow} refreshFn={refreshFn} />
        </div>

        <div className="flex-1">
          <DAGStatus workflow={workflow} fileName={name || ''} />
        </div>
      </div>
    </WorkflowContext.Provider>
  );
};

export default WorkflowDetailsContent;
