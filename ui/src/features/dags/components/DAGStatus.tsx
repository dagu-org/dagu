import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, NodeStatus, Status } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useClient } from '../../../hooks/api';
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
  const navigate = useNavigate();
  const [modal, setModal] = useState(false);
  const [selectedStep, setSelectedStep] = useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  // State for log viewer
  const [logViewer, setLogViewer] = useState({
    isOpen: false,
    logType: 'step' as 'execution' | 'step',
    stepName: '',
    workflowId: '',
    stream: 'stdout' as 'stdout' | 'stderr',
  });
  const client = useClient();
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    // Check if this is a child workflow by checking if rootWorkflowId and rootWorkflowName exist
    // and are different from the current workflow's ID and name
    const isChildWorkflow =
      workflow.rootWorkflowId &&
      workflow.rootWorkflowName &&
      workflow.rootWorkflowId !== workflow.workflowId;

    // Define path parameters with proper typing
    const pathParams = {
      name: isChildWorkflow ? workflow.rootWorkflowName : workflow.name,
      workflowId: isChildWorkflow
        ? workflow.rootWorkflowId
        : workflow.workflowId,
      stepName: step.name,
      ...(isChildWorkflow ? { childWorkflowId: workflow.workflowId } : {}),
    };

    // Use the appropriate endpoint based on whether this is a child workflow
    const endpoint = isChildWorkflow
      ? '/workflows/{name}/{workflowId}/children/{childWorkflowId}/steps/{stepName}/status'
      : '/workflows/{name}/{workflowId}/steps/{stepName}/status';

    const { error } = await client.PATCH(endpoint, {
      params: {
        path: pathParams,
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
      body: {
        status,
      },
    });
    if (error) {
      alert(error.message || 'An error occurred');
      return;
    }
    dismissModal();
  };
  // Handle double-click on graph node (navigate to child workflow)
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      // find the clicked step
      const n = workflow.nodes?.find(
        (n) => n.step.name.replace(/\s/g, '_') == id
      );

      if (n && n.step.run) {
        // Find the child workflow ID
        const childWorkflow = n.children?.[0];

        if (childWorkflow && childWorkflow.workflowId) {
          // Navigate to the child workflow status page
          const workflowId = workflow.rootWorkflowId;

          // Check if we're in a workflow context or a DAG context
          // More reliable detection by checking the current URL path or the workflow object
          const currentPath = window.location.pathname;
          const isModal =
            document.querySelector('.workflow-modal-content') !== null;
          const isWorkflowContext =
            currentPath.startsWith('/workflows/') || isModal;
          if (isWorkflowContext) {
            // For workflows, use query parameters to navigate to the workflow details page
            const searchParams = new URLSearchParams();
            searchParams.set('childWorkflowId', childWorkflow.workflowId);

            // Use root workflow information
            if (workflow.rootWorkflowId) {
              searchParams.set('workflowId', workflow.rootWorkflowId);
              searchParams.set('workflowName', workflow.rootWorkflowName);
            } else {
              searchParams.set('workflowId', workflow.workflowId);
              searchParams.set('workflowName', workflow.name);
            }

            searchParams.set('step', n.step.name);
            navigate(`/workflows/${workflow.name}?${searchParams.toString()}`);
          } else {
            // For DAGs, use the existing approach with query parameters
            navigate({
              pathname: `/dags/${fileName}`,
              search: `?childWorkflowId=${childWorkflow.workflowId}&workflowId=${workflowId}&step=${n.step.name}&workflowName=${encodeURIComponent(workflow.rootWorkflowName)}`,
            });
          }
        }
      }
    },
    [workflow, navigate, fileName]
  );

  // Handle right-click on graph node (show status update modal)
  const onRightClickStepOnGraph = React.useCallback(
    (id: string) => {
      const status = workflow.status;

      // Only allow status updates for completed workflows
      if (status !== Status.Running && status !== Status.NotStarted) {
        // find the right-clicked step
        const n = workflow.nodes?.find(
          (n) => n.step.name.replace(/\s/g, '_') == id
        );

        if (n) {
          // Show the modal (it will be centered by default)
          setSelectedStep(n.step);
          setModal(true);
        }
      }
    },
    [workflow]
  );

  const handlers = getEventHandlers(workflow);

  // Handler for opening log viewer
  const handleViewLog = (stepName: string, workflowId: string) => {
    // Check if this is a stderr log (indicated by _stderr suffix)
    const isStderr = stepName.endsWith('_stderr');
    const actualStepName = isStderr ? stepName.slice(0, -7) : stepName; // Remove '_stderr' suffix

    setLogViewer({
      isOpen: true,
      logType: 'step',
      stepName: actualStepName,
      workflowId: workflowId || workflow.workflowId,
      stream: isStderr ? 'stderr' : 'stdout',
    });
  };

  return (
    <div className="space-y-6">
      {/* DAG Visualization Card */}
      <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
        <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
            Workflow Visualization
          </h2>
        </div>
        <div className="p-6">
          <DAGGraph
            workflow={workflow}
            onSelectStep={onSelectStepOnGraph}
            onRightClickStep={onRightClickStepOnGraph}
          />
        </div>
      </div>

      <DAGContext.Consumer>
        {(props) => (
          <>
            <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
              {/* Status Overview Card */}
              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                  <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                    Execution Status
                  </h2>
                </div>
                <div className="p-6">
                  <DAGStatusOverview
                    status={workflow}
                    fileName={fileName}
                    onViewLog={(workflowId) => {
                      setLogViewer({
                        isOpen: true,
                        logType: 'execution',
                        stepName: '',
                        workflowId,
                        stream: 'stdout',
                      });
                    }}
                  />
                </div>
              </div>

              {/* Steps Table Card */}
              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden xl:col-span-2">
                <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                  <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 flex items-center justify-between">
                    <span>Execution Steps</span>
                    {workflow.nodes && (
                      <span className="text-sm font-normal text-slate-500 dark:text-slate-400">
                        {workflow.nodes.length} step{workflow.nodes.length !== 1 ? 's' : ''}
                      </span>
                    )}
                  </h2>
                </div>
                <div className="overflow-x-auto">
                  <NodeStatusTable
                    nodes={workflow.nodes}
                    status={workflow}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                </div>
              </div>
            </div>
            
            {/* Lifecycle Hooks Card */}
            {handlers?.length ? (
              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                  <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 flex items-center justify-between">
                    <span>Lifecycle Hooks</span>
                    <span className="text-sm font-normal text-slate-500 dark:text-slate-400">
                      {handlers.length} hook{handlers.length !== 1 ? 's' : ''}
                    </span>
                  </h2>
                </div>
                <div className="overflow-x-auto">
                  <NodeStatusTable
                    nodes={handlers}
                    status={workflow}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                </div>
              </div>
            ) : null}
          </>
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
        workflow={workflow}
        stream={logViewer.stream}
      />
    </div>
  );
}

export default DAGStatus;
