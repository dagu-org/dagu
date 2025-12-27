import React, { useState } from 'react';
import { ActivitySquare, Package } from 'lucide-react';
import { Tabs } from '@/components/ui/tabs';
import { components, Status } from '../../../../api/v2/schema';
import { DAGStatus } from '../../../../features/dags/components';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import { useHasOutputs } from '../../hooks/useHasOutputs';
import DAGRunHeader from './DAGRunHeader';
import DAGRunOutputs from './DAGRunOutputs';
import ModalLinkTab from '../../../../features/dags/components/common/ModalLinkTab';

type DAGRunDetailsContentProps = {
  name: string;
  dagRun: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
  dagRunId?: string;
  isSubDAGRun?: boolean;
  parentName?: string;
  parentDagRunId?: string;
};

const DAGRunDetailsContent: React.FC<DAGRunDetailsContentProps> = ({
  name,
  dagRun,
  refreshFn,
  dagRunId = 'latest',
  isSubDAGRun = false,
  parentName,
  parentDagRunId,
}) => {
  const [activeTab, setActiveTab] = useState<'status' | 'outputs'>('status');

  // Use actual dagRunId from dagRun, not the prop which may be "latest"
  const actualDagRunId = dagRun.dagRunId;

  // Check if outputs exist for conditional tab display
  const hasOutputs = useHasOutputs(
    dagRun.name || name || '',
    actualDagRunId,
    dagRun.status as Status,
    isSubDAGRun,
    parentName,
    parentDagRunId
  );

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

        {/* Tabs - only show if outputs exist */}
        {hasOutputs && (
          <div className="mb-4">
            <Tabs className="whitespace-nowrap">
              <ModalLinkTab
                label="Status"
                value="status"
                isActive={activeTab === 'status'}
                icon={ActivitySquare}
                onClick={() => setActiveTab('status')}
              />
              <ModalLinkTab
                label="Outputs"
                value="outputs"
                isActive={activeTab === 'outputs'}
                icon={Package}
                onClick={() => setActiveTab('outputs')}
              />
            </Tabs>
          </div>
        )}

        <div className="flex-1">
          {activeTab === 'status' || !hasOutputs ? (
            <DAGStatus dagRun={dagRun} fileName={name || ''} />
          ) : (
            <DAGRunOutputs
              dagName={dagRun.name || name || ''}
              dagRunId={actualDagRunId}
              isSubDAGRun={isSubDAGRun}
              parentName={parentName}
              parentDagRunId={parentDagRunId}
            />
          )}
        </div>
      </div>
    </DAGRunContext.Provider>
  );
};

export default DAGRunDetailsContent;
