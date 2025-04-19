import React from 'react';
import { DAGContext } from '../contexts/DAGContext';
import { getEventHandlers } from '../lib/getEventHandlers';
import { NodeStatusTable, DAGStatusOverview } from './dag-details';
import { StatusUpdateModal } from './dag-execution';
import { Box } from '@mui/material';
import SubTitle from '../../../ui/SubTitle';
import { components, NodeStatus, Status } from '../../../api/v2/schema';
import { DAGGraph } from './visualization';
import { useClient } from '../../../hooks/api';
import { AppBarContext } from '../../../contexts/AppBarContext';

type Props = {
  run: components['schemas']['RunDetails'];
  fileId: string;
};

function DAGStatus({ run, fileId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [modal, setModal] = React.useState(false);
  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  const client = useClient();
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    const { error } = await client.PATCH(
      '/runs/{dagName}/{requestId}/{stepName}/status',
      {
        params: {
          path: {
            dagName: run.name,
            requestId: run.requestId,
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
    mutate(['/dags/{fileId}']);
    mutate(['/dags/{fileId}/runs']);
    dismissModal();
  };
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const status = run.status;
      if (status == Status.Running || status == Status.NotStarted) {
        return;
      }
      // find the clicked step
      const n = run.nodes.find((n) => n.step.name.replace(/\s/g, '_') == id);
      if (n) {
        setSelectedStep(n.step);
        setModal(true);
      }
    },
    [run]
  );

  const handlers = getEventHandlers(run);

  return (
    <React.Fragment>
      <DAGGraph run={run} onSelectStep={onSelectStepOnGraph} />
      <Box>
        <DAGContext.Consumer>
          {(props) => (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <Box sx={{ mt: 2 }}>
                  <DAGStatusOverview
                    status={run}
                    fileId={fileId}
                  ></DAGStatusOverview>
                </Box>
              </Box>

              <Box sx={{ mt: 3 }}>
                <SubTitle>Steps</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={run.nodes}
                    status={run}
                    {...props}
                  ></NodeStatusTable>
                </Box>
              </Box>

              {handlers?.length ? (
                <Box sx={{ mt: 3 }}>
                  <SubTitle>Lifecycle Hooks</SubTitle>
                  <Box sx={{ mt: 2 }}>
                    <NodeStatusTable
                      nodes={handlers}
                      status={run}
                      {...props}
                    ></NodeStatusTable>
                  </Box>
                </Box>
              ) : null}
            </React.Fragment>
          )}
        </DAGContext.Consumer>
      </Box>

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </React.Fragment>
  );
}

export default DAGStatus;
