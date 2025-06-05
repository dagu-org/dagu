import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, NodeStatus, Status } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useConfig } from '../../../contexts/ConfigContext';
import { useClient } from '../../../hooks/api';
import { DAGContext } from '../contexts/DAGContext';
import { getEventHandlers } from '../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from './dag-details';
import { LogViewer, StatusUpdateModal } from './dag-execution';
import { DAGGraph } from './visualization';

type Props = {
  dagRun: components['schemas']['DAGRunDetails'];
  fileName: string;
};

function DAGStatus({ dagRun, fileName }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
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
    dagRunId: '',
    stream: 'stdout' as 'stdout' | 'stderr',
  });
  const client = useClient();
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    // Check if this is a child DAG-run by checking if rootDAGRunId and rootDAGRunName exist
    // and are different from the current DAG-run's ID and name
    const isChildDAGRun =
      dagRun.rootDAGRunId &&
      dagRun.rootDAGRunName &&
      dagRun.rootDAGRunId !== dagRun.dagRunId;

    // Define path parameters with proper typing
    const pathParams = {
      name: isChildDAGRun ? dagRun.rootDAGRunName : dagRun.name,
      dagRunId: isChildDAGRun ? dagRun.rootDAGRunId : dagRun.dagRunId,
      stepName: step.name,
      ...(isChildDAGRun ? { childDAGRunId: dagRun.dagRunId } : {}),
    };

    // Use the appropriate endpoint based on whether this is a child DAG-run
    const endpoint = isChildDAGRun
      ? '/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}/steps/{stepName}/status'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/status';

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
  // Handle double-click on graph node (navigate to child dagRun)
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      // find the clicked step
      const n = dagRun.nodes?.find(
        (n) => n.step.name.replace(/\s/g, '_') == id
      );

      if (n && n.step.run) {
        // Find the child dagRun ID
        const childDAGRun = n.children?.[0];

        if (childDAGRun && childDAGRun.dagRunId) {
          // Navigate to the child DAG-run status page
          const dagRunId = dagRun.rootDAGRunId;

          // Check if we're in a dagRun context or a DAG context
          // More reliable detection by checking the current URL path or the dagRun object
          const currentPath = window.location.pathname;
          const isModal =
            document.querySelector('.dagRun-modal-content') !== null;
          const isDAGRunContext =
            currentPath.startsWith('/dag-runs/') || isModal;
          if (isDAGRunContext) {
            // For DAG runs, use query parameters to navigate to the DAG-run details page
            const searchParams = new URLSearchParams();
            searchParams.set('childDAGRunId', childDAGRun.dagRunId);

            // Use root DAG-run information
            if (dagRun.rootDAGRunId) {
              searchParams.set('dagRunId', dagRun.rootDAGRunId);
              searchParams.set('dagRunName', dagRun.rootDAGRunName);
            } else {
              searchParams.set('dagRunId', dagRun.dagRunId);
              searchParams.set('dagRunName', dagRun.name);
            }

            searchParams.set('step', n.step.name);
            navigate(
              `/dag-runs/${dagRun.name}/${dagRunId}?${searchParams.toString()}`
            );
          } else {
            // For DAGs, use the existing approach with query parameters
            navigate({
              pathname: `/dags/${fileName}`,
              search: `?childDAGRunId=${childDAGRun.dagRunId}&dagRunId=${dagRunId}&step=${n.step.name}&dagRunName=${encodeURIComponent(dagRun.rootDAGRunName)}`,
            });
          }
        }
      }
    },
    [dagRun, navigate, fileName]
  );

  // Handle right-click on graph node (show status update modal)
  const onRightClickStepOnGraph = React.useCallback(
    (id: string) => {
      // Check if user has permission to run DAGs
      if (!config.permissions.runDags) {
        return;
      }

      const status = dagRun.status;

      // Only allow status updates for completed DAG runs
      if (status !== Status.Running && status !== Status.NotStarted) {
        // find the right-clicked step
        const n = dagRun.nodes?.find(
          (n) => n.step.name.replace(/[-\s]/g, 'dagutmp') == id
        );

        if (n) {
          // Show the modal (it will be centered by default)
          setSelectedStep(n.step);
          setModal(true);
        }
      }
    },
    [dagRun, config.permissions.runDags]
  );

  const handlers = getEventHandlers(dagRun);

  // Handler for opening log viewer
  const handleViewLog = (stepName: string, dagRunId: string) => {
    // Check if this is a stderr log (indicated by _stderr suffix)
    const isStderr = stepName.endsWith('_stderr');
    const actualStepName = isStderr ? stepName.slice(0, -7) : stepName; // Remove '_stderr' suffix

    setLogViewer({
      isOpen: true,
      logType: 'step',
      stepName: actualStepName,
      dagRunId: dagRunId || dagRun.dagRunId,
      stream: isStderr ? 'stderr' : 'stdout',
    });
  };

  return (
    <div className="space-y-6">
      {/* DAG Visualization Card */}
      <div className="bg-card rounded-2xl border border-border shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
        <div className="border-b border-border bg-muted/30 px-6 py-4">
          <h2 className="text-lg font-semibold text-foreground">
            Graph
          </h2>
        </div>
        <div className="p-6">
          <DAGGraph
            dagRun={dagRun}
            onSelectStep={onSelectStepOnGraph}
            onRightClickStep={onRightClickStepOnGraph}
          />
        </div>
      </div>

      <DAGContext.Consumer>
        {(props) => (
          <>
            <div className="grid grid-cols-1 gap-6">
              {/* Status Overview Card */}
              <div className="bg-card rounded-2xl border border-border shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-border bg-muted/30 px-6 py-4">
                  <h2 className="text-lg font-semibold text-foreground">
                    Run Status
                  </h2>
                </div>
                <div className="p-6">
                  <DAGStatusOverview
                    status={dagRun}
                    fileName={fileName}
                    onViewLog={(dagRunId) => {
                      setLogViewer({
                        isOpen: true,
                        logType: 'execution',
                        stepName: '',
                        dagRunId,
                        stream: 'stdout',
                      });
                    }}
                  />
                </div>
              </div>

              {/* Desktop Steps Table Card */}
              <div className="hidden md:block bg-card rounded-2xl border border-border shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-border bg-muted/30 px-6 py-4">
                  <h2 className="text-lg font-semibold text-foreground flex items-center justify-between">
                    <span>Steps</span>
                    {dagRun.nodes && (
                      <span className="text-sm font-normal text-muted-foreground">
                        {dagRun.nodes.length} step
                        {dagRun.nodes.length !== 1 ? 's' : ''}
                      </span>
                    )}
                  </h2>
                </div>
                <div className="overflow-x-auto">
                  <NodeStatusTable
                    nodes={dagRun.nodes}
                    status={dagRun}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                </div>
              </div>

              {/* Mobile Steps - No Card Container */}
              <div className="md:hidden">
                <div className="mb-4">
                  <h2 className="text-lg font-semibold text-foreground flex items-center justify-between">
                    <span>Steps</span>
                    {dagRun.nodes && (
                      <span className="text-sm font-normal text-muted-foreground">
                        {dagRun.nodes.length} step
                        {dagRun.nodes.length !== 1 ? 's' : ''}
                      </span>
                    )}
                  </h2>
                </div>
                <NodeStatusTable
                  nodes={dagRun.nodes}
                  status={dagRun}
                  {...props}
                  onViewLog={handleViewLog}
                />
              </div>
            </div>

            {/* Lifecycle Hooks */}
            {handlers?.length ? (
              <>
                {/* Desktop Lifecycle Hooks Card */}
                <div className="hidden md:block bg-card rounded-2xl border border-border shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                  <div className="border-b border-border bg-muted/30 px-6 py-4">
                    <h2 className="text-lg font-semibold text-foreground flex items-center justify-between">
                      <span>Lifecycle Hooks</span>
                      <span className="text-sm font-normal text-muted-foreground">
                        {handlers.length} hook{handlers.length !== 1 ? 's' : ''}
                      </span>
                    </h2>
                  </div>
                  <div className="overflow-x-auto">
                    <NodeStatusTable
                      nodes={handlers}
                      status={dagRun}
                      {...props}
                      onViewLog={handleViewLog}
                    />
                  </div>
                </div>

                {/* Mobile Lifecycle Hooks - No Card Container */}
                <div className="md:hidden">
                  <div className="mb-4">
                    <h2 className="text-lg font-semibold text-foreground flex items-center justify-between">
                      <span>Lifecycle Hooks</span>
                      <span className="text-sm font-normal text-muted-foreground">
                        {handlers.length} hook{handlers.length !== 1 ? 's' : ''}
                      </span>
                    </h2>
                  </div>
                  <NodeStatusTable
                    nodes={handlers}
                    status={dagRun}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                </div>
              </>
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
        dagName={dagRun.name}
        dagRunId={logViewer.dagRunId}
        stepName={logViewer.stepName}
        dagRun={dagRun}
        stream={logViewer.stream}
      />
    </div>
  );
}

export default DAGStatus;
