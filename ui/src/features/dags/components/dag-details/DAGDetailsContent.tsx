import { Tabs } from '@/components/ui/tabs';
import {
  ActivitySquare,
  FileCode,
  History,
  Package,
  ScrollText,
} from 'lucide-react';
import React, { useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { DAGRunOutputs } from '../../../dag-runs/components/dag-run-details';
import { useHasOutputs } from '../../../dag-runs/hooks/useHasOutputs';
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
  currentDAGRun: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  activeTab: string;
  onTabChange?: (tab: string) => void;
  dagRunId?: string;
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
  currentDAGRun,
  refreshFn,
  formatDuration,
  activeTab,
  onTabChange,
  dagRunId = 'latest',
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

  // Check if outputs exist for conditional tab display
  // Use actual dagRunId from currentDAGRun, not the URL param which may be "latest"
  const actualDagRunId = currentDAGRun.dagRunId;
  const hasOutputs = useHasOutputs(
    dag?.name || '',
    actualDagRunId,
    currentDAGRun.status
  );

  const handleTabClick = (tab: string) => {
    if (onTabChange) {
      onTabChange(tab);
    }

    // Open log viewer when clicking on log tabs
    if (tab === 'dagRun-log') {
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
      <div className="w-full h-full flex flex-col">
        {/* Only render the header if skipHeader is not true */}
        {!skipHeader && (
          <DAGHeader
            dag={dag}
            currentDAGRun={currentDAGRun}
            fileName={fileName || ''}
            refreshFn={refreshFn}
            formatDuration={formatDuration}
            navigateToStatusTab={navigateToStatusTab}
          />
        )}
        <div className="flex flex-col lg:flex-row justify-between items-start lg:items-center gap-3 lg:gap-0 mb-4">
          {/* Desktop Tabs (lg and up) */}
          <div className="hidden lg:block overflow-x-auto">
            <Tabs className="whitespace-nowrap">
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

              {/* Outputs Tab - Only show when outputs exist */}
              {hasOutputs &&
                (isModal ? (
                  <ModalLinkTab
                    label="Outputs"
                    value="outputs"
                    isActive={activeTab === 'outputs'}
                    icon={Package}
                    onClick={() => handleTabClick('outputs')}
                  />
                ) : (
                  <LinkTab
                    label="Outputs"
                    value={`${baseUrl}/outputs`}
                    isActive={activeTab === 'outputs'}
                    icon={Package}
                  />
                ))}

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

              {(activeTab === 'log' || activeTab === 'dagRun-log') &&
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

          {/* Mobile/Tablet Tabs (sm to lg) */}
          <div className="lg:hidden w-full overflow-x-auto">
            <div className="flex space-x-1 w-full">
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

              {/* Outputs Tab - Only show when outputs exist */}
              {hasOutputs &&
                (isModal ? (
                  <ModalLinkTab
                    label=""
                    value="outputs"
                    isActive={activeTab === 'outputs'}
                    icon={Package}
                    onClick={() => handleTabClick('outputs')}
                    className="flex-1 justify-center"
                  />
                ) : (
                  <LinkTab
                    label=""
                    value={`${baseUrl}/outputs`}
                    isActive={activeTab === 'outputs'}
                    icon={Package}
                    className="flex-1 justify-center"
                  />
                ))}

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

              {(activeTab === 'log' || activeTab === 'dagRun-log') &&
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
        <div className="flex-1 flex flex-col min-h-0">
          {activeTab === 'status' ? (
            <DAGStatus dagRun={currentDAGRun} fileName={fileName || ''} />
          ) : null}
          {activeTab === 'outputs' ? (
            <DAGRunOutputs dagName={dag?.name || ''} dagRunId={actualDagRunId} />
          ) : null}
          {activeTab === 'spec' ? <DAGSpec fileName={fileName} /> : null}
          {activeTab === 'history' ? (
            <div data-tab="history">
              <DAGExecutionHistory fileName={fileName || ''} />
            </div>
          ) : null}
          {activeTab === 'dagRun-log' ? (
            <ExecutionLog
              name={dag?.name || ''}
              dagRunId={dagRunId}
              dagRun={currentDAGRun}
            />
          ) : null}
          {activeTab === 'log' && stepName ? (
            <StepLog
              dagName={dag?.name || ''}
              dagRunId={dagRunId}
              stepName={stepName}
              dagRun={currentDAGRun}
            />
          ) : null}

          {/* Log viewer modal */}
          <LogViewer
            isOpen={logViewer.isOpen}
            onClose={closeLogViewer}
            logType={logViewer.logType}
            dagName={dag?.name || ''}
            dagRunId={dagRunId}
            stepName={logViewer.stepName}
            isInModal={isModal}
            dagRun={currentDAGRun}
          />
        </div>
      </div>
    </DAGContext.Provider>
  );
};

export default DAGDetailsContent;
