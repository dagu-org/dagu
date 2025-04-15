import { Box } from '@mui/material';
import React, { useMemo } from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import { getEventHandlers } from '../../models';
import NodeStatusTable from '../molecules/NodeStatusTable';
import DAGStatusOverview from '../molecules/DAGStatusOverview';
import SubTitle from '../atoms/SubTitle';
import LoadingIndicator from '../atoms/LoadingIndicator';
import HistoryTable from '../molecules/HistoryTable';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import { components, NodeStatus, Status } from '../../api/v2/schema';
import { useClient, useMutate, useQuery } from '../../hooks/api';
import { AppBarContext } from '../../contexts/AppBarContext';
import DAGGraph from '../molecules/DAGGraph';
import StatusUpdateModal from '../molecules/StatusUpdateModal';

type Props = {
  fileId: string;
};

function DAGExecutionHistory({ fileId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
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
    { refreshInterval: 2000 }
  );
  if (!data) {
    return <LoadingIndicator />;
  }
  if (!data.runs?.length) {
    return <Box>Execution history was not found.</Box>;
  }
  return <DAGHistoryTable runs={data.runs} gridData={data.gridData} />;
}

type HistoryTableProps = {
  gridData: components['schemas']['DAGGridItem'][] | null;
  runs: components['schemas']['RunDetails'][] | null;
};

function DAGHistoryTable({ gridData, runs }: HistoryTableProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const mutate = useMutate();
  const [modal, setModal] = React.useState(false);
  const idxParam = new URLSearchParams(window.location.search).get('idx');
  const [idx, setIdx] = React.useState(
    idxParam ? parseInt(idxParam) : runs && runs.length ? runs.length - 1 : 0
  );
  const dagStatusContext = React.useContext(RunDetailsContext);
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

  let handlers: components['schemas']['Node'][] | null = null;
  if (runs && idx < runs.length && runs[idx]) {
    handlers = getEventHandlers(runs[idx]);
  }
  const reversedRuns = useMemo(() => {
    return [...(runs || [])].reverse();
  }, [runs]);

  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);

  const dismissModal = () => setModal(false);

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
    mutate(['/dags/{fileId}']);
    mutate(['/dags/{fileId}/runs']);
    dismissModal();
  };

  React.useEffect(() => {
    if (reversedRuns && reversedRuns[idx]) {
      dagStatusContext.setData(reversedRuns[idx]);
    }
  }, [reversedRuns, idx]);

  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const run = reversedRuns[idx];
      if (!run) {
        return;
      }
      if (run.status == Status.Running || run.status == Status.NotStarted) {
        return;
      }
      // find the clicked step
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
          <Box>
            <SubTitle>Execution History</SubTitle>
            <HistoryTable
              runs={reversedRuns || []}
              gridData={gridData || []}
              onSelect={updateIdx}
              idx={idx}
            />
          </Box>

          {reversedRuns && reversedRuns[idx] ? (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <DAGGraph
                  run={reversedRuns[idx]}
                  onSelectStep={onSelectStepOnGraph}
                />
                <Box sx={{ mt: 2 }}>
                  <SubTitle>Status</SubTitle>
                  <DAGStatusOverview
                    status={reversedRuns[idx]}
                    requestId={reversedRuns[idx].requestId}
                    {...props}
                  />
                </Box>
              </Box>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Steps</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={reversedRuns[idx].nodes}
                    status={reversedRuns[idx]}
                    {...props}
                  />
                </Box>
              </Box>

              {handlers && handlers.length ? (
                <Box sx={{ mt: 3 }}>
                  <SubTitle>Lifecycle Hooks</SubTitle>
                  <Box sx={{ mt: 2 }}>
                    <NodeStatusTable
                      nodes={getEventHandlers(reversedRuns[idx])}
                      status={reversedRuns[idx]}
                      {...props}
                    />
                  </Box>
                </Box>
              ) : null}
            </React.Fragment>
          ) : null}

          <StatusUpdateModal
            visible={modal}
            step={setSelectedStep}
            dismissModal={dismissModal}
            onSubmit={onUpdateStatus}
          />
        </React.Fragment>
      )}
    </DAGContext.Consumer>
  );
}

export default DAGExecutionHistory;
