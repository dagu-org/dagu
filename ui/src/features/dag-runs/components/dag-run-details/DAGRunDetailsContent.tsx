import React from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGStatus } from '../../../../features/dags/components';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import DAGRunHeader from './DAGRunHeader';

type DAGRunDetailsContentProps = {
  name: string;
  dagRun: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
  dagRunId?: string;
};

const DAGRunDetailsContent: React.FC<DAGRunDetailsContentProps> = ({
  name,
  dagRun,
  refreshFn,
  dagRunId = 'latest',
}) => {
  return (
    <DAGRunContext.Provider
      value={{
        refresh: refreshFn,
        name: name || '',
        dagRunId: dagRunId || '',
      }}
    >
      <div className="w-full flex flex-col">
        {/* Display breadcrumbs and DAG-run details in the header */}
        <DAGRunHeader dagRun={dagRun} refreshFn={refreshFn} />

        <div className="flex-1">
          <DAGStatus dagRun={dagRun} fileName={name || ''} />
        </div>
      </div>
    </DAGRunContext.Provider>
  );
};

export default DAGRunDetailsContent;
