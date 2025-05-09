import React, { useState } from 'react';
import { components, NodeStatus, Status } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useClient } from '../../../hooks/api';
import SubTitle from '../../../ui/SubTitle';
import { DAGContext } from '../contexts/DAGContext';
import { getEventHandlers } from '../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from './dag-details';
import { LogViewer, StatusUpdateModal } from './dag-execution';
import { DAGGraph } from './visualization';

type Props = {
  workflow: components['schemas']['WorkflowDetails'];
  fileName: string;
};

function DAGStatus({ workflow, fileName }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [modal, setModal] = useState(false);
  const [selectedStep, setSelectedStep] = useState<
    components['schemas']['Step'] | undefined
  >(undefined);

  console.log('DAGStatus', workflow);
  // State for log viewer
  const [logViewer, setLogViewer] = useState({
    isOpen: false,
    logType: 'step' as 'execution' | 'step',
    stepName: '',
    workflowId: '',
  });
  const client = useClient();
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    const { error } = await client.PATCH(
      '/workflows/{name}/{workflowId}/steps/{stepName}/status',
      {
        params: {
          path: {
            name: workflow.name,
            workflowId: workflow.workflowId,
            stepName: step.name,
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          status,
        },
      }
    );
    if (error) {
      alert(error.message || 'An error occurred');
      return;
    }
    dismissModal();
  };
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const status = workflow.status;
      if (status == Status.Running || status == Status.NotStarted) {
        return;
      }
      // find the clicked step
      const n = workflow.nodes?.find(
        (n) => n.step.name.replace(/\s/g, '_') == id
      );
      if (n) {
        setSelectedStep(n.step);
        setModal(true);
      }
    },
    [workflow]
  );

  const handlers = getEventHandlers(workflow);

  // Handler for opening log viewer
  const handleViewLog = (stepName: string, workflowId: string) => {
    setLogViewer({
      isOpen: true,
      logType: 'step',
      stepName,
      workflowId: workflowId || workflow.workflowId,
    });
  };

  return (
    <div className="space-y-4">
      <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
        <DAGGraph workflow={workflow} onSelectStep={onSelectStepOnGraph} />
      </div>

      <DAGContext.Consumer>
        {(props) => (
          <React.Fragment>
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
              <SubTitle className="mb-4">Status</SubTitle>
              <DAGStatusOverview
                status={workflow}
                fileName={fileName}
                onViewLog={(workflowId) => {
                  setLogViewer({
                    isOpen: true,
                    logType: 'execution',
                    stepName: '',
                    workflowId,
                  });
                }}
              />
            </div>

            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
              <SubTitle className="mb-4">Steps</SubTitle>
              <NodeStatusTable
                nodes={workflow.nodes}
                status={workflow}
                {...props}
                onViewLog={handleViewLog}
              />
            </div>

            {handlers?.length ? (
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                <SubTitle className="mb-4">Lifecycle Hooks</SubTitle>
                <NodeStatusTable
                  nodes={handlers}
                  status={workflow}
                  {...props}
                  onViewLog={handleViewLog}
                />
              </div>
            ) : null}
          </React.Fragment>
        )}
      </DAGContext.Consumer>

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />

      {/* Log viewer modal */}
      <LogViewer
        isOpen={logViewer.isOpen}
        onClose={() => setLogViewer((prev) => ({ ...prev, isOpen: false }))}
        logType={logViewer.logType}
        dagName={workflow.name}
        workflowId={logViewer.workflowId}
        stepName={logViewer.stepName}
      />
    </div>
  );
}

export default DAGStatus;
