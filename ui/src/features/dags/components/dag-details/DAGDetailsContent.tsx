import { Tabs } from '@/components/ui/tabs';
import { ActivitySquare, FileCode, History, ScrollText } from 'lucide-react';
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGStatus } from '../../components';
import { DAGContext } from '../../contexts/DAGContext';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { LinkTab } from '../common';
import ModalLinkTab from '../common/ModalLinkTab';
import { DAGEditButtons, DAGSpec } from '../dag-editor';
import { DAGExecutionHistory, ExecutionLog, StepLog } from '../dag-execution';
import { DAGHeader } from './';

type DAGDetailsContentProps = {
  fileId: string;
  dag: components['schemas']['DAG'];
  latestRun: components['schemas']['RunDetails'];
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  activeTab: string;
  onTabChange?: (tab: string) => void;
  requestId?: string;
  stepName?: string | null;
  isModal?: boolean;
};

const DAGDetailsContent: React.FC<DAGDetailsContentProps> = ({
  fileId,
  dag,
  latestRun,
  refreshFn,
  formatDuration,
  activeTab,
  onTabChange,
  requestId = 'latest',
  stepName = null,
  isModal = false,
}) => {
  const baseUrl = isModal ? '#' : `/dags/${fileId}`;

  const handleTabClick = (tab: string) => {
    if (onTabChange) {
      onTabChange(tab);
    }
  };

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileId: fileId || '',
        name: dag?.name || '',
      }}
    >
      <RunDetailsContext.Provider
        value={{
          data: latestRun,
          setData: () => {}, // This will be overridden by the parent component
        }}
      >
        <div className="w-full flex flex-col">
          <DAGHeader
            dag={dag}
            latestRun={latestRun}
            fileId={fileId || ''}
            refreshFn={refreshFn}
            formatDuration={formatDuration}
          />
          <div className="my-4 flex flex-row justify-between items-center">
            <Tabs
              value={activeTab}
              className="bg-white p-1.5 rounded-lg shadow-sm border border-gray-100/80"
            >
              {isModal ? (
                <ModalLinkTab
                  label="Status"
                  value="status"
                  isActive={activeTab === 'status'}
                  icon={ActivitySquare}
                  onClick={() => handleTabClick('status')}
                />
              ) : (
                <LinkTab
                  label="Status"
                  value={`${baseUrl}`}
                  isActive={activeTab === 'status'}
                  icon={ActivitySquare}
                />
              )}

              {isModal ? (
                <ModalLinkTab
                  label="Spec"
                  value="spec"
                  isActive={activeTab === 'spec'}
                  icon={FileCode}
                  onClick={() => handleTabClick('spec')}
                />
              ) : (
                <LinkTab
                  label="Spec"
                  value={`${baseUrl}/spec`}
                  isActive={activeTab === 'spec'}
                  icon={FileCode}
                />
              )}

              {isModal ? (
                <ModalLinkTab
                  label="History"
                  value="history"
                  isActive={activeTab === 'history'}
                  icon={History}
                  onClick={() => handleTabClick('history')}
                />
              ) : (
                <LinkTab
                  label="History"
                  value={`${baseUrl}/history`}
                  isActive={activeTab === 'history'}
                  icon={History}
                />
              )}

              {(activeTab === 'log' || activeTab === 'scheduler-log') &&
                (isModal ? (
                  <ModalLinkTab
                    label="Log"
                    value={activeTab}
                    isActive={true}
                    icon={ScrollText}
                    onClick={() => {}}
                  />
                ) : (
                  <LinkTab
                    label="Log"
                    value={baseUrl}
                    isActive={true}
                    icon={ScrollText}
                  />
                ))}
            </Tabs>
            {activeTab === 'spec' ? (
              <DAGEditButtons fileId={fileId || ''} />
            ) : null}
          </div>
          <div className="flex-1">
            {activeTab === 'status' ? (
              <DAGStatus run={latestRun} fileId={fileId || ''} />
            ) : null}
            {activeTab === 'spec' ? <DAGSpec fileId={fileId} /> : null}
            {activeTab === 'history' ? (
              <DAGExecutionHistory fileId={fileId || ''} />
            ) : null}
            {activeTab === 'scheduler-log' ? (
              <ExecutionLog name={dag?.name || ''} requestId={requestId} />
            ) : null}
            {activeTab === 'log' && stepName ? (
              <StepLog
                dagName={dag?.name || ''}
                requestId={requestId}
                stepName={stepName}
              />
            ) : null}
          </div>
        </div>
      </RunDetailsContext.Provider>
    </DAGContext.Provider>
  );
};

export default DAGDetailsContent;
