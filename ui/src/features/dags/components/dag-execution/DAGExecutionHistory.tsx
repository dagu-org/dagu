/**
 * DAGExecutionHistory component displays the execution history of a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import React, { useMemo, useState } from 'react';
import { components, NodeStatus, Status } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient, useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import SubTitle from '../../../../ui/SubTitle';
import { DAGContext } from '../../contexts/DAGContext';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { getEventHandlers } from '../../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from '../dag-details';
import { DAGGraph } from '../visualization';
import { HistoryTable, LogViewer, StatusUpdateModal } from './';

/**
 * Props for the DAGExecutionHistory component
 */
type Props = {
  /** DAG file ID */
  fileId: string;
  /** Whether the component is rendered in a modal */
  isInModal?: boolean;
  /** The active tab in the parent component */
  activeTab?: string;
};

/**
 * DAGExecutionHistory displays the execution history of a DAG
 * including a history table, graph visualization, and status details
 */
function DAGExecutionHistory({ fileId, isInModal, activeTab }: Props) {
  const appBarContext = React.useContext(AppBarContext);

  // Fetch execution history data
  const { data } = useQuery(
    '/dags/{fileId}/runs',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: fileId,
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
  if (!data.runs?.length) {
    return <div>Execution history was not found.</div>;
  }

  return <DAGHistoryTable runs={data.runs} gridData={data.gridData} />;
}

/**
 * Props for the DAGHistoryTable component
 */
type HistoryTableProps = {
  /** Grid data for visualization */
  gridData: components['schemas']['DAGGridItem'][] | null;
  /** List of DAG runs */
  runs: components['schemas']['RunDetails'][] | null;
};

/**
 * DAGHistoryTable displays detailed execution history with interactive elements
 */
function DAGHistoryTable({ gridData, runs }: HistoryTableProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const [modal, setModal] = React.useState(false);

  // State for log viewer
  const [logViewer, setLogViewer] = useState({
    isOpen: false,
    logType: 'step' as 'execution' | 'step',
    stepName: '',
    requestId: '',
  });

  // Get the selected run index from URL parameters
  const idxParam = new URLSearchParams(window.location.search).get('idx');
  const [idx, setIdx] = React.useState(
    idxParam ? parseInt(idxParam) : runs && runs.length ? runs.length - 1 : 0
  );

  const dagStatusContext = React.useContext(RunDetailsContext);

  // Ensure index is valid when runs change (e.g., when switching DAGs)
  React.useEffect(() => {
    if (!runs || runs.length === 0) return;

    // Clamp the index to be within valid range
    const maxIdx = runs.length - 1;
    const validIdx = Math.max(0, Math.min(idx, maxIdx));

    // Only update if the index needs adjustment
    if (validIdx !== idx) {
      const params = new URLSearchParams(window.location.search);
      params.set('idx', validIdx.toString());
      window.history.replaceState(
        {},
        '',
        `${window.location.pathname}?${params}`
      );
      setIdx(validIdx);
    }
  }, [runs, idx]);

  /**
   * Update the selected run index and update URL parameters
   */
  const updateIdx = (newIdx: number) => {
    // Ensure newIdx is within valid range
    if (newIdx < 0 || !runs || newIdx >= runs.length) {
      return;
    }

    setIdx(newIdx);
    const params = new URLSearchParams(window.location.search);
    params.set('idx', newIdx.toString());
    window.history.replaceState(
      {},
      '',
      `${window.location.pathname}?${params}`
    );
  };

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
    [idx, runs]
  );

  // Add and remove keyboard event listener
  React.useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  // Get event handlers for the selected run
  let handlers: components['schemas']['Node'][] | null = null;
  if (runs && idx < runs.length && runs[idx]) {
    handlers = getEventHandlers(runs[idx]);
  }

  // Reverse the runs array for display (newest first)
  const reversedRuns = useMemo(() => {
    return [...(runs || [])].reverse();
  }, [runs]);

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
      !reversedRuns ||
      idx >= reversedRuns.length ||
      !reversedRuns[idx]
    ) {
      return;
    }

    // Call the API to update the step status
    const { error } = await client.PATCH(
      '/runs/{dagName}/{requestId}/{stepName}/status',
      {
        params: {
          path: {
            dagName: reversedRuns[idx].name,
            requestId: reversedRuns[idx].requestId,
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

  // Update the DAG status context when the selected run changes
  React.useEffect(() => {
    if (reversedRuns && reversedRuns[idx]) {
      dagStatusContext.setData(reversedRuns[idx]);
    }
  }, [reversedRuns, idx]);

  /**
   * Handle step selection on the graph
   */
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const run = reversedRuns[idx];
      if (!run) {
        return;
      }

      // Only allow status updates for completed runs
      if (run.status == Status.Running || run.status == Status.NotStarted) {
        return;
      }

      // Find the clicked step
      const n = run.nodes?.find((n) => n.step.name.replace(/\s/g, '_') == id);
      if (n) {
        setSelectedStep(n.step);
        setModal(true);
      }
    },
    [reversedRuns, idx]
  );

  return (
    <DAGContext.Consumer>
      {(props) => (
        <div className="space-y-4">
          <div className="mb-2">
            <HistoryTable
              runs={reversedRuns || []}
              gridData={gridData || []}
              onSelect={updateIdx}
              idx={idx}
            />
          </div>

          {reversedRuns && reversedRuns[idx] ? (
            <React.Fragment>
              <div className="space-y-6 pt-2">
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                  <DAGGraph
                    run={reversedRuns[idx]}
                    onSelectStep={onSelectStepOnGraph}
                  />
                </div>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                <SubTitle className="mb-4">Status</SubTitle>
                <DAGStatusOverview
                  status={reversedRuns[idx]}
                  requestId={reversedRuns[idx].requestId}
                  {...props}
                  onViewLog={(requestId) => {
                    setLogViewer({
                      isOpen: true,
                      logType: 'execution',
                      stepName: '',
                      requestId,
                    });
                  }}
                />
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                <SubTitle className="mb-4">Steps</SubTitle>
                <NodeStatusTable
                  nodes={reversedRuns[idx].nodes}
                  status={reversedRuns[idx]}
                  {...props}
                  onViewLog={(stepName, requestId) => {
                    setLogViewer({
                      isOpen: true,
                      logType: 'step',
                      stepName,
                      requestId:
                        requestId || reversedRuns[idx]?.requestId || '',
                    });
                  }}
                />
              </div>

              {handlers && handlers.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
                  <SubTitle className="mb-4">Lifecycle Hooks</SubTitle>
                  <NodeStatusTable
                    nodes={getEventHandlers(reversedRuns[idx])}
                    status={reversedRuns[idx]}
                    {...props}
                    onViewLog={(stepName, requestId) => {
                      setLogViewer({
                        isOpen: true,
                        logType: 'step',
                        stepName,
                        requestId:
                          requestId || reversedRuns[idx]?.requestId || '',
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
                      reversedRuns && reversedRuns[idx]
                        ? reversedRuns[idx].name
                        : ''
                    }
                    requestId={logViewer.requestId}
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
