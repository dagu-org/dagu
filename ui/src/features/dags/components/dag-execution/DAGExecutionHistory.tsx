/**
 * DAGExecutionHistory component displays the execution history of a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import { useErrorModal } from '@/components/ui/error-modal';
import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { components, NodeStatus, Status, Stream } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient, useQuery } from '../../../../hooks/api';
import { useDAGHistorySSE } from '../../../../hooks/useDAGHistorySSE';
import { toMermaidNodeId } from '../../../../lib/utils';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { getEventHandlers } from '../../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from '../dag-details';
import { DAGGraph } from '../visualization';
import { HistoryTable, LogViewer, StatusUpdateModal } from './';

/**
 * Props for the DAGExecutionHistory component
 */
type Props = {
  /** DAG file name */
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

  // SSE for real-time updates with polling fallback
  const sseResult = useDAGHistorySSE(fileName, !!fileName);
  const shouldPoll = sseResult.shouldUseFallback || !sseResult.isConnected;

  // Fetch execution history data - use polling only as fallback
  const { data: pollingData } = useQuery(
    '/dags/{fileName}/dag-runs',
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
    {
      refreshInterval: shouldPoll ? 2000 : 0,
      keepPreviousData: true,
      isPaused: () => !shouldPoll && sseResult.isConnected,
    }
  );

  // Use SSE data when available, otherwise fall back to polling
  const data = sseResult.data || pollingData;

  // Show loading indicator while fetching data
  if (!data) {
    return <LoadingIndicator />;
  }

  // Show message if no execution history is found
  if (!data.dagRuns?.length) {
    return <div>Execution history was not found.</div>;
  }

  return (
    <DAGHistoryTable
      fileName={fileName}
      dagRuns={data.dagRuns}
      gridData={data.gridData}
    />
  );
}

/**
 * Props for the DAGHistoryTable component
 */
type HistoryTableProps = {
  /** DAG file name */
  fileName: string;
  /** Grid data for visualization */
  gridData: components['schemas']['DAGGridItem'][] | null;
  /** List of DAG dagRuns */
  dagRuns: components['schemas']['DAGRunDetails'][] | null;
};

/**
 * DAGHistoryTable displays detailed execution history with interactive elements
 */
function DAGHistoryTable({ fileName, gridData, dagRuns }: HistoryTableProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const navigate = useNavigate();
  const { showError } = useErrorModal();
  const [modal, setModal] = React.useState(false);

  // State for log viewer
  const [logViewer, setLogViewer] = useState<{
    isOpen: boolean;
    logType: 'execution' | 'step';
    stepName: string;
    dagRunId: string;
    stream: Stream;
  }>({
    isOpen: false,
    logType: 'step',
    stepName: '',
    dagRunId: '',
    stream: Stream.stdout,
  });

  // Get the selected dagRun index from URL parameters using React Router
  const [searchParams, setSearchParams] = useSearchParams();
  const idxParam = searchParams.get('idx');
  const [idx, setIdx] = React.useState(
    idxParam
      ? parseInt(idxParam)
      : dagRuns && dagRuns.length
        ? dagRuns.length - 1
        : 0
  );

  // Removed unused context since we're no longer directly updating it

  // Ensure index is valid when dagRuns change (e.g., when switching DAGs)
  React.useEffect(() => {
    if (!dagRuns || dagRuns.length === 0) return;

    // Clamp the index to be within valid range
    const maxIdx = dagRuns.length - 1;
    const validIdx = Math.max(0, Math.min(idx, maxIdx));

    // Only update if the index needs adjustment
    if (validIdx !== idx) {
      const newParams = new URLSearchParams(searchParams);
      newParams.set('idx', validIdx.toString());
      setSearchParams(newParams);
      setIdx(validIdx);
    }
  }, [dagRuns, idx]);

  /**
   * Update the selected dagRun index and update URL parameters
   */
  const updateIdx = (newIdx: number) => {
    // Ensure newIdx is within valid range
    if (newIdx < 0 || !dagRuns || newIdx >= dagRuns.length) {
      return;
    }

    setIdx(newIdx);
    const reversedDAGRuns = [...(dagRuns || [])].reverse();

    if (reversedDAGRuns && reversedDAGRuns[newIdx]) {
      // Instead of directly updating the context, update the URL with the dagRun ID
      const selectedDAGRun = reversedDAGRuns[newIdx];
      const newParams = new URLSearchParams(searchParams);
      newParams.set('idx', newIdx.toString());

      // Add or update the dagRunId parameter
      newParams.set('dagRunId', selectedDAGRun.dagRunId);

      // Add dagRunName parameter to avoid waiting for DAG details
      newParams.set('dagRunName', selectedDAGRun.name);

      setSearchParams(newParams);
    }
  };

  // Listen for URL parameter changes
  useEffect(() => {
    if (idxParam) {
      const newIdx = parseInt(idxParam);
      if (!isNaN(newIdx) && newIdx !== idx) {
        setIdx(newIdx);

        // No longer updating the RootDAGRunContext here
        // The status details page will handle this based on URL parameters
      }
    }
  }, [idxParam, idx]);

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
    [idx, dagRuns, updateIdx]
  );

  // Add and remove keyboard event listener
  React.useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  // Get event handlers for the selected dagRun
  let handlers: components['schemas']['Node'][] | null = null;
  if (dagRuns && idx < dagRuns.length && dagRuns[idx]) {
    handlers = getEventHandlers(dagRuns[idx]);
  }

  // Reverse the dagRuns array for display (newest first)
  const reversedDAGRuns = [...(dagRuns || [])].reverse();

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
    _step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    if (
      !selectedStep ||
      !reversedDAGRuns ||
      idx >= reversedDAGRuns.length ||
      !reversedDAGRuns[idx]
    ) {
      return;
    }

    // Call the API to update the step status
    const { error } = await client.PATCH(
      '/dag-runs/{name}/{dagRunId}/steps/{stepName}/status',
      {
        params: {
          path: {
            name: reversedDAGRuns[idx].name,
            dagRunId: reversedDAGRuns[idx].dagRunId,
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
      showError(
        error.message || 'Failed to update status',
        'Please try again or check the server connection.'
      );
      return;
    }

    dismissModal();
  };

  // Removed the effect that updates the DAG status context
  // The status details page will handle this based on URL parameters

  /**
   * Handle double-click on graph node (navigate to sub dagRun)
   */
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const dagRun = reversedDAGRuns[idx];
      if (!dagRun) {
        return;
      }

      // Find the clicked step
      const n = dagRun.nodes?.find(
        (n) => toMermaidNodeId(n.step.name) == id
      );
      if (!n) return;

      // If it's a sub dagRun, navigate to its details
      const subRuns = [...(n.subRuns ?? []), ...(n.subRunsRepeated ?? [])];

      // Check for sub-DAG: step.call (for call steps) OR subRun.dagName (for chat tools, etc.)
      const subDAGName = n.step?.call || subRuns[0]?.dagName;
      if (!subDAGName || subRuns.length === 0) return;

      const subDAGRun = subRuns[0];
      if (subDAGRun && subDAGRun.dagRunId) {
        // Navigate to the sub dagRun details using React Router with search params
        // Include dagRunName parameter to avoid waiting for DAG details
        navigate({
          pathname: `/dags/${fileName}`,
          search: `?dagRunId=${dagRun.rootDAGRunId}&subDAGRunId=${subDAGRun.dagRunId}&dagRunName=${encodeURIComponent(dagRun.rootDAGRunName)}`,
        });
      }
    },
    [reversedDAGRuns, idx, navigate]
  );

  /**
   * Handle right-click on graph node (show status update modal)
   */
  const onRightClickStepOnGraph = React.useCallback(
    (id: string) => {
      const dagRun = reversedDAGRuns[idx];
      if (!dagRun) {
        return;
      }

      // Only allow status updates for completed dagRuns
      if (
        dagRun.status == Status.Running ||
        dagRun.status == Status.NotStarted
      ) {
        return;
      }

      // Find the right-clicked step
      const n = dagRun.nodes?.find(
        (n) => toMermaidNodeId(n.step.name) == id
      );

      if (n) {
        setSelectedStep(n.step);
        setModal(true);
      }
    },
    [reversedDAGRuns, idx]
  );

  return (
    <DAGContext.Consumer>
      {(props) => (
        <div className="space-y-6">
          <HistoryTable
            dagRuns={reversedDAGRuns || []}
            gridData={gridData || []}
            onSelect={updateIdx}
            idx={idx}
          />

          {reversedDAGRuns && reversedDAGRuns[idx] ? (
            <React.Fragment>
              <DAGGraph
                dagRun={reversedDAGRuns[idx]}
                onSelectStep={onSelectStepOnGraph}
                onRightClickStep={onRightClickStepOnGraph}
              />

              <div className="bg-surface border border-border rounded-lg p-4">
                <DAGStatusOverview
                  status={reversedDAGRuns[idx]}
                  onViewLog={(dagRunId) => {
                    setLogViewer({
                      isOpen: true,
                      logType: 'execution',
                      stepName: '',
                      dagRunId,
                      stream: Stream.stdout,
                    });
                  }}
                />
              </div>

              <NodeStatusTable
                nodes={reversedDAGRuns[idx].nodes}
                status={reversedDAGRuns[idx]}
                {...props}
                onViewLog={(stepName, dagRunId) => {
                  const isStderr = stepName.endsWith('_stderr');
                  const actualStepName = isStderr
                    ? stepName.slice(0, -7)
                    : stepName;

                  setLogViewer({
                    isOpen: true,
                    logType: 'step',
                    stepName: actualStepName,
                    dagRunId:
                      dagRunId || reversedDAGRuns[idx]?.dagRunId || '',
                    stream: isStderr ? Stream.stderr : Stream.stdout,
                  });
                }}
              />

              {handlers && handlers.length ? (
                <NodeStatusTable
                  nodes={getEventHandlers(reversedDAGRuns[idx])}
                  status={reversedDAGRuns[idx]}
                  {...props}
                  onViewLog={(stepName, dagRunId) => {
                    const isStderr = stepName.endsWith('_stderr');
                    const actualStepName = isStderr
                      ? stepName.slice(0, -7)
                      : stepName;

                    setLogViewer({
                      isOpen: true,
                      logType: 'step',
                      stepName: actualStepName,
                      dagRunId:
                        dagRunId || reversedDAGRuns[idx]?.dagRunId || '',
                      stream: isStderr ? Stream.stderr : Stream.stdout,
                    });
                  }}
                />
              ) : null}

              {/* Log viewer modal - moved outside to handle all log viewing */}
              <LogViewer
                isOpen={logViewer.isOpen}
                onClose={() =>
                  setLogViewer((prev) => ({ ...prev, isOpen: false }))
                }
                logType={logViewer.logType}
                dagName={
                  reversedDAGRuns && reversedDAGRuns[idx]
                    ? reversedDAGRuns[idx].name
                    : ''
                }
                dagRunId={logViewer.dagRunId}
                stepName={logViewer.stepName}
                dagRun={reversedDAGRuns[idx]}
                stream={logViewer.stream}
              />
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
