/**
 * DAGExecutionHistory component displays the execution history of a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import React, { useMemo } from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import { getEventHandlers } from '../../lib/getEventHandlers';
import SubTitle from '../../../../ui/SubTitle';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { components, NodeStatus, Status } from '../../../../api/v2/schema';
import { useClient, useQuery } from '../../../../hooks/api';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { DAGGraph } from '../visualization';
import { NodeStatusTable } from '../dag-details';
import { DAGStatusOverview } from '../dag-details';
import { HistoryTable } from './';
import { StatusUpdateModal } from './';
import { cn } from '@/lib/utils';

/**
 * Props for the DAGExecutionHistory component
 */
type Props = {
  /** DAG file ID */
  fileId: string;
};

/**
 * DAGExecutionHistory displays the execution history of a DAG
 * including a history table, graph visualization, and status details
 */
function DAGExecutionHistory({ fileId }: Props) {
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

  // Get the selected run index from URL parameters
  const idxParam = new URLSearchParams(window.location.search).get('idx');
  const [idx, setIdx] = React.useState(
    idxParam ? parseInt(idxParam) : runs && runs.length ? runs.length - 1 : 0
  );

  const dagStatusContext = React.useContext(RunDetailsContext);

  /**
   * Update the selected run index and update URL parameters
   */
  const updateIdx = (newIdx: number) => {
    setIdx(newIdx);
    const params = new URLSearchParams(window.location.search);
    params.set('idx', newIdx.toString());
    window.history.replaceState(
      {},
      '',
      `${window.location.pathname}?${params}`
    );
  };

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
      const n = run.nodes.find((n) => n.step.name.replace(/\s/g, '_') == id);
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
        <React.Fragment>
          <HistoryTable
            runs={reversedRuns || []}
            gridData={gridData || []}
            onSelect={updateIdx}
            idx={idx}
          />
          {reversedRuns && reversedRuns[idx] ? (
            <React.Fragment>
              <div className="mt-6">
                <DAGGraph
                  run={reversedRuns[idx]}
                  onSelectStep={onSelectStepOnGraph}
                />
                <div className="mt-4">
                  <SubTitle>Status</SubTitle>
                  <DAGStatusOverview
                    status={reversedRuns[idx]}
                    requestId={reversedRuns[idx].requestId}
                    {...props}
                  />
                </div>
              </div>
              <div className="mt-6">
                <SubTitle>Steps</SubTitle>
                <div className="mt-4">
                  <NodeStatusTable
                    nodes={reversedRuns[idx].nodes}
                    status={reversedRuns[idx]}
                    {...props}
                  />
                </div>
              </div>

              {handlers && handlers.length ? (
                <div className="mt-6">
                  <SubTitle>Lifecycle Hooks</SubTitle>
                  <div className="mt-4">
                    <NodeStatusTable
                      nodes={getEventHandlers(reversedRuns[idx])}
                      status={reversedRuns[idx]}
                      {...props}
                    />
                  </div>
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
        </React.Fragment>
      )}
    </DAGContext.Consumer>
  );
}

export default DAGExecutionHistory;
