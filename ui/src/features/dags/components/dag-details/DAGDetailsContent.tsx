import { Tabs } from '@/components/ui/tabs';
import { ActivitySquare, FileCode, History, ScrollText } from 'lucide-react';
import React, { useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGStatus } from '../../components';
import { DAGContext } from '../../contexts/DAGContext';
import { LinkTab } from '../common';
import ModalLinkTab from '../common/ModalLinkTab';
import { DAGEditButtons, DAGSpec } from '../dag-editor';
import {
  DAGExecutionHistory,
  ExecutionLog,
  LogViewer,
  StepLog,
} from '../dag-execution';
import { DAGHeader } from './';

type DAGDetailsContentProps = {
  fileName: string;
  dag: components['schemas']['DAG'];
  currentWorkflow: components['schemas']['WorkflowDetails'];
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  activeTab: string;
  onTabChange?: (tab: string) => void;
  workflowId?: string;
  stepName?: string | null;
  isModal?: boolean;
  navigateToStatusTab?: () => void;
  skipHeader?: boolean; // Add this prop to optionally skip rendering the header
};

type LogViewerState = {
  isOpen: boolean;
  logType: 'execution' | 'step';
  stepName?: string;
};

const DAGDetailsContent: React.FC<DAGDetailsContentProps> = ({
  fileName,
  dag,
  currentWorkflow,
  refreshFn,
  formatDuration,
  activeTab,
  onTabChange,
  workflowId = 'latest',
  stepName = null,
  isModal = false,
  navigateToStatusTab,
  skipHeader = false,
}) => {
  const baseUrl = isModal ? '#' : `/dags/${fileName}`;
  const [logViewer, setLogViewer] = useState<LogViewerState>({
    isOpen: false,
    logType: 'execution',
    stepName: undefined,
  });

  const handleTabClick = (tab: string) => {
    if (onTabChange) {
      onTabChange(tab);
    }

    // Open log viewer when clicking on log tabs
    if (tab === 'workflow-log') {
      setLogViewer({
        isOpen: true,
        logType: 'execution',
      });
    } else if (tab === 'log' && stepName) {
      setLogViewer({
        isOpen: true,
        logType: 'step',
        stepName,
      });
    }
  };

  const closeLogViewer = () => {
    setLogViewer((prev) => ({ ...prev, isOpen: false }));
  };

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileName: fileName || '',
        name: dag?.name || '',
      }}
    >
      <div className="w-full flex flex-col">
        {/* Only render the header if skipHeader is not true */}
        {!skipHeader && (
          <DAGHeader
            dag={dag}
            currentWorkflow={currentWorkflow}
            fileName={fileName || ''}
            refreshFn={refreshFn}
            formatDuration={formatDuration}
            navigateToStatusTab={navigateToStatusTab}
          />
        )}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-3 md:gap-0 mb-4">
          {/* Desktop Tabs (md and up) */}
          <div className="hidden md:block overflow-x-auto">
            <Tabs className="bg-white p-1.5 rounded-lg shadow-sm border border-gray-100/80 whitespace-nowrap">
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

              {(activeTab === 'log' || activeTab === 'workflow-log') &&
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
          </div>

          {/* Mobile/Tablet Tabs (sm to md) */}
          <div className="md:hidden w-full overflow-x-auto">
            <div className="flex space-x-1 bg-white p-1.5 rounded-lg shadow-sm border border-gray-100/80 w-full">
              {isModal ? (
                <ModalLinkTab
                  label=""
                  value="status"
                  isActive={activeTab === 'status'}
                  icon={ActivitySquare}
                  onClick={() => handleTabClick('status')}
                  className="flex-1 justify-center"
                />
              ) : (
                <LinkTab
                  label=""
                  value={`${baseUrl}`}
                  isActive={activeTab === 'status'}
                  icon={ActivitySquare}
                  className="flex-1 justify-center"
                />
              )}

              {isModal ? (
                <ModalLinkTab
                  label=""
                  value="spec"
                  isActive={activeTab === 'spec'}
                  icon={FileCode}
                  onClick={() => handleTabClick('spec')}
                  className="flex-1 justify-center"
                />
              ) : (
                <LinkTab
                  label=""
                  value={`${baseUrl}/spec`}
                  isActive={activeTab === 'spec'}
                  icon={FileCode}
                  className="flex-1 justify-center"
                />
              )}

              {isModal ? (
                <ModalLinkTab
                  label=""
                  value="history"
                  isActive={activeTab === 'history'}
                  icon={History}
                  onClick={() => handleTabClick('history')}
                  className="flex-1 justify-center"
                />
              ) : (
                <LinkTab
                  label=""
                  value={`${baseUrl}/history`}
                  isActive={activeTab === 'history'}
                  icon={History}
                  className="flex-1 justify-center"
                />
              )}

              {(activeTab === 'log' || activeTab === 'workflow-log') &&
                (isModal ? (
                  <ModalLinkTab
                    label=""
                    value={activeTab}
                    isActive={true}
                    icon={ScrollText}
                    onClick={() => {}}
                    className="flex-1 justify-center"
                  />
                ) : (
                  <LinkTab
                    label=""
                    value={baseUrl}
                    isActive={true}
                    icon={ScrollText}
                    className="flex-1 justify-center"
                  />
                ))}
            </div>
          </div>

          {activeTab === 'spec' ? (
            <DAGEditButtons fileName={fileName || ''} />
          ) : null}
        </div>
        <div className="flex-1">
          {activeTab === 'status' ? (
            <DAGStatus workflow={currentWorkflow} fileName={fileName || ''} />
          ) : null}
          {activeTab === 'spec' ? <DAGSpec fileName={fileName} /> : null}
          {activeTab === 'history' ? (
            <div data-tab="history">
              <DAGExecutionHistory fileName={fileName || ''} />
            </div>
          ) : null}
          {activeTab === 'workflow-log' ? (
            <ExecutionLog
              name={dag?.name || ''}
              workflowId={workflowId}
              workflow={currentWorkflow}
            />
          ) : null}
          {activeTab === 'log' && stepName ? (
            <StepLog
              dagName={dag?.name || ''}
              workflowId={workflowId}
              stepName={stepName}
              workflow={currentWorkflow}
            />
          ) : null}

          {/* Log viewer modal */}
          <LogViewer
            isOpen={logViewer.isOpen}
            onClose={closeLogViewer}
            logType={logViewer.logType}
            dagName={dag?.name || ''}
            workflowId={workflowId}
            stepName={logViewer.stepName}
            isInModal={isModal}
            workflow={currentWorkflow}
          />
        </div>
      </div>
    </DAGContext.Provider>
  );
};

export default DAGDetailsContent;
