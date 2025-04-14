import React from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStatus } from '../../models';
import { getEventHandlers } from '../../models';
import NodeStatusTable from '../molecules/NodeStatusTable';
import DAGStatusOverview from '../molecules/DAGStatusOverview';
import { useDAGPostAPI } from '../../hooks/useDAGPostAPI';
import StatusUpdateModal from '../molecules/StatusUpdateModal';
import { Box } from '@mui/material';
import SubTitle from '../atoms/SubTitle';
import { components, Status } from '../../api/v2/schema';
import DAGGraph from '../molecules/DAGGraph';

type Props = {
  run: components['schemas']['RunDetails'];
  location: string;
  refresh: () => void;
};

function DAGStatus({ run, location, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  const { doPost } = useDAGPostAPI({
    name: location,
    onSuccess: refresh,
    requestId: run.requestId,
  });
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    action: string
  ) => {
    doPost(action, step.name);
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
                    location={location}
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
