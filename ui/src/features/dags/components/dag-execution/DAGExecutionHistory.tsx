/**
 * DAGExecutionHistory component displays the execution history of a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { components, NodeStatus, Status } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient, useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import SubTitle from '../../../../ui/SubTitle';
import { DAGContext } from '../../contexts/DAGContext';
import { RootWorkflowContext } from '../../contexts/RootWorkflowContext';
import { getEventHandlers } from '../../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from '../dag-details';
import { DAGGraph } from '../visualization';
import { HistoryTable, LogViewer, StatusUpdateModal } from './';

/**
 * Props for the DAGExecutionHistory component
 */
type Props = {
  /** DAG file ID */
  fileName: string;
  /** Whether the component is rendered in a modal */
  isInModal?: boolean;
  /** The active tab in the parent component */
  activeTab?: string;
};

/**
 * DAGExecutionHistory displays the execution history of a DAG
 * including a history table, graph visualization, and status details
 */
function DAGExecutionHistory({
  fileName,
}: Omit<Props, 'isInModal' | 'activeTab'>) {
  const appBarContext = React.useContext(AppBarContext);

  // Fetch execution history data
  const { data } = useQuery(
    '/dags/{fileName}/workflows',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileName: fileName,
        },
      },
    },
    { refreshInterval: 2000 } // Refresh every 2 seconds
  );

  // Show loading indicator while fetching data
  if (!data) {
    return <LoadingIndicator />;
  }

  // Show message if no execution history is found
  if (!data.workflows?.length) {
    return <div>Execution history was not found.</div>;
  }

  return (
    <DAGHistoryTable
      fileName={fileName}
      workflows={data.workflows}
      gridData={data.gridData}
    />
  );
}

/**
 * Props for the DAGHistoryTable component
 */
type HistoryTableProps = {
  /** DAG file ID */
  fileName: string;
  /** Grid data for visualization */
  gridData: components['schemas']['DAGGridItem'][] | null;
  /** List of DAG workflows */
  workflows: components['schemas']['WorkflowDetails'][] | null;
};

/**
 * DAGHistoryTable displays detailed execution history with interactive elements
 */
function DAGHistoryTable({ fileName, gridData, workflows }: HistoryTableProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const navigate = useNavigate();
  const [modal, setModal] = React.useState(false);

  // State for log viewer
  const [logViewer, setLogViewer] = useState({
    isOpen: false,
    logType: 'step' as 'execution' | 'step',
    stepName: '',
    workflowId: '',
  });

  // Get the selected workflow index from URL parameters using React Router
  const [searchParams, setSearchParams] = useSearchParams();
  const idxParam = searchParams.get('idx');
  const [idx, setIdx] = React.useState(
    idxParam
      ? parseInt(idxParam)
      : workflows && workflows.length
        ? workflows.length - 1
        : 0
  );

  const dagStatusContext = React.useContext(RootWorkflowContext);

  // Ensure index is valid when workflows change (e.g., when switching DAGs)
  React.useEffect(() => {
    if (!workflows || workflows.length === 0) return;

    // Clamp the index to be within valid range
    const maxIdx = workflows.length - 1;
    const validIdx = Math.max(0, Math.min(idx, maxIdx));

    // Only update if the index needs adjustment
    if (validIdx !== idx) {
      const newParams = new URLSearchParams(searchParams);
      newParams.set('idx', validIdx.toString());
      setSearchParams(newParams);
      setIdx(validIdx);
    }
  }, [workflows, idx]);

  /**
   * Update the selected workflow index and update URL parameters
   */
  const updateIdx = (newIdx: number) => {
    // Ensure newIdx is within valid range
    if (newIdx < 0 || !workflows || newIdx >= workflows.length) {
      return;
    }

    setIdx(newIdx);
    const newParams = new URLSearchParams(searchParams);
    newParams.set('idx', newIdx.toString());
    setSearchParams(newParams);

    // Directly update the RootWorkflowContext when a history item is selected
    // This ensures the header will update immediately
    const reversedWorkflows = [...(workflows || [])].reverse();
    if (reversedWorkflows && reversedWorkflows[newIdx]) {
      dagStatusContext.setData(reversedWorkflows[newIdx]);
    }
  };

  // Listen for URL parameter changes
  useEffect(() => {
    if (idxParam) {
      const newIdx = parseInt(idxParam);
      if (!isNaN(newIdx) && newIdx !== idx) {
        setIdx(newIdx);

        // Update the RootWorkflowContext when the URL parameter changes
        const reversedWorkflows = [...(workflows || [])].reverse();
        if (reversedWorkflows && reversedWorkflows[newIdx]) {
          dagStatusContext.setData(reversedWorkflows[newIdx]);
        }
      }
    }
  }, [idxParam, workflows, dagStatusContext]);

  /**
   * Handle keyboard navigation with arrow keys
   */
  const handleKeyDown = React.useCallback(
    (event: KeyboardEvent) => {
      if (event.key === 'ArrowLeft') {
        // Navigate to previous history item
        updateIdx(idx - 1);
      } else if (event.key === 'ArrowRight') {
        // Navigate to next history item
        updateIdx(idx + 1);
      }
    },
    [idx, workflows, updateIdx]
  );

  // Add and remove keyboard event listener
  React.useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  // Get event handlers for the selected workflow
  let handlers: components['schemas']['Node'][] | null = null;
  if (workflows && idx < workflows.length && workflows[idx]) {
    handlers = getEventHandlers(workflows[idx]);
  }

  // Reverse the workflows array for display (newest first)
  const reversedWorkflows = [...(workflows || [])].reverse();

  // State for the selected step in the status update modal
  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);

  /**
   * Close the status update modal
   */
  const dismissModal = () => setModal(false);

  /**
   * Update the status of a step
   */
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    if (
      !selectedStep ||
      !reversedWorkflows ||
      idx >= reversedWorkflows.length ||
      !reversedWorkflows[idx]
    ) {
      return;
    }

    // Call the API to update the step status
    const { error } = await client.PATCH(
      '/workflows/{name}/{workflowId}/steps/{stepName}/status',
      {
        params: {
          path: {
            name: reversedWorkflows[idx].name,
            workflowId: reversedWorkflows[idx].workflowId,
            stepName: selectedStep.name,
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

  // Update the DAG status context when the selected workflow changes
  React.useEffect(() => {
    if (reversedWorkflows && reversedWorkflows[idx]) {
      dagStatusContext.setData(reversedWorkflows[idx]);
    }
  }, [reversedWorkflows, idx, dagStatusContext]);

  /**
   * Handle step selection on the graph
   */
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const workflow = reversedWorkflows[idx];
      if (!workflow) {
        return;
      }

      // Find the clicked step
      const n = workflow.nodes?.find(
        (n) => n.step.name.replace(/\s/g, '_') == id
      );

      if (!n) return;

      // Check if this is a child workflow node (has a 'run' property)
      const isChildWorkflow = !!n.step.run;

      if (isChildWorkflow) {
        // If it's a child workflow, navigate to its details
        const childWorkflow = n.children?.[0];
        if (childWorkflow && childWorkflow.workflowId && n.step.run) {
          // Navigate to the child workflow details using React Router with search params
          // Include workflowName parameter to avoid waiting for DAG details
          navigate({
            pathname: `/dags/${fileName}`,
            search: `?workflowId=${workflow.rootWorkflowId}&childWorkflowId=${childWorkflow.workflowId}&workflowName=${encodeURIComponent(workflow.rootWorkflowName)}`,
          });
          return;
        }
      }

      // For non-child workflow nodes or if child workflow navigation fails,
      // only allow status updates for completed workflows
      if (
        workflow.status == Status.Running ||
        workflow.status == Status.NotStarted
      ) {
        return;
      }

      setSelectedStep(n.step);
      setModal(true);
    },
    [reversedWorkflows, idx, navigate]
  );

  return (
    <DAGContext.Consumer>
      {(props) => (
        <div className="space-y-4">
          <div className="mb-2">
            <HistoryTable
              workflows={reversedWorkflows || []}
              gridData={gridData || []}
              onSelect={updateIdx}
              idx={idx}
            />
          </div>

          {reversedWorkflows && reversedWorkflows[idx] ? (
            <React.Fragment>
              <div className="space-y-6 pt-2">
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                  <DAGGraph
                    workflow={reversedWorkflows[idx]}
                    onSelectStep={onSelectStepOnGraph}
                  />
                </div>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                <SubTitle className="mb-4">Status</SubTitle>
                <DAGStatusOverview
                  status={reversedWorkflows[idx]}
                  workflowId={reversedWorkflows[idx].workflowId}
                  {...props}
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

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                <SubTitle className="mb-4">Steps</SubTitle>
                <NodeStatusTable
                  nodes={reversedWorkflows[idx].nodes}
                  status={reversedWorkflows[idx]}
                  {...props}
                  onViewLog={(stepName, workflowId) => {
                    setLogViewer({
                      isOpen: true,
                      logType: 'step',
                      stepName,
                      workflowId:
                        workflowId || reversedWorkflows[idx]?.workflowId || '',
                    });
                  }}
                />
              </div>

              {handlers && handlers.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                  <SubTitle className="mb-4">Lifecycle Hooks</SubTitle>
                  <NodeStatusTable
                    nodes={getEventHandlers(reversedWorkflows[idx])}
                    status={reversedWorkflows[idx]}
                    {...props}
                    onViewLog={(stepName, workflowId) => {
                      setLogViewer({
                        isOpen: true,
                        logType: 'step',
                        stepName,
                        workflowId:
                          workflowId ||
                          reversedWorkflows[idx]?.workflowId ||
                          '',
                      });
                    }}
                  />

                  {/* Log viewer modal */}
                  <LogViewer
                    isOpen={logViewer.isOpen}
                    onClose={() =>
                      setLogViewer((prev) => ({ ...prev, isOpen: false }))
                    }
                    logType={logViewer.logType}
                    dagName={
                      reversedWorkflows && reversedWorkflows[idx]
                        ? reversedWorkflows[idx].name
                        : ''
                    }
                    workflowId={logViewer.workflowId}
                    stepName={logViewer.stepName}
                  />
                </div>
              ) : null}
            </React.Fragment>
          ) : null}

          <StatusUpdateModal
            visible={modal}
            step={selectedStep}
            dismissModal={dismissModal}
            onSubmit={onUpdateStatus}
          />
        </div>
      )}
    </DAGContext.Consumer>
  );
}

export default DAGExecutionHistory;
