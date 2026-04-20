import React from 'react';
import { components } from '../../../../api/v1/schema';
import { DAGStatus } from '../../../../features/dags/components';
import type { StatusTab } from '../../../../features/dags/components/DAGStatus';
import { cn } from '../../../../lib/utils';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import DAGRunHeader from './DAGRunHeader';

type DAGRunDetailsContentProps = {
  name: string;
  dagRun: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
  dagRunId?: string;
  initialTab?: StatusTab;
  fillHeight?: boolean;
};

const DAGRunDetailsContent: React.FC<DAGRunDetailsContentProps> = ({
  name,
  dagRun,
  refreshFn,
  dagRunId = 'latest',
  initialTab = 'status',
  fillHeight = false,
}) => {
  return (
    <DAGRunContext.Provider
      value={{
        refresh: refreshFn,
        name: name || '',
        dagRunId: dagRunId || '',
      }}
    >
      <div
        className={cn('flex w-full flex-col', fillHeight && 'h-full min-h-0')}
      >
        {/* Display breadcrumbs and DAG-run details in the header */}
        <DAGRunHeader dagRun={dagRun} refreshFn={refreshFn} />

        <div className={cn('flex-1', fillHeight && 'min-h-0')}>
          <DAGStatus
            dagRun={dagRun}
            fileName={name || ''}
            initialTab={initialTab}
            fillHeight={fillHeight}
          />
        </div>
      </div>
    </DAGRunContext.Provider>
  );
};

export default DAGRunDetailsContent;
