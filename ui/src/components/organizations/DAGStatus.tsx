import React from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStatus } from '../../models';
import { getEventHandlers } from '../../models';
import Graph, { FlowchartType } from '../molecules/Graph';
import NodeStatusTable from '../molecules/NodeStatusTable';
import DAGStatusOverview from '../molecules/DAGStatusOverview';
import TimelineChart from '../molecules/TimelineChart';
import { useDAGPostAPI } from '../../hooks/useDAGPostAPI';
import StatusUpdateModal from '../molecules/StatusUpdateModal';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import SubTitle from '../atoms/SubTitle';
import BorderedBox from '../atoms/BorderedBox';
import { useCookies } from 'react-cookie';
import FlowchartSwitch from '../molecules/FlowchartSwitch';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChartGantt, faShareNodes } from '@fortawesome/free-solid-svg-icons';
import { components, Status } from '../../api/v2/schema';

type Props = {
  run: components['schemas']['RunDetails'];
  name: string;
  refresh: () => void;
};

function DAGStatus({ run, name, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [sub, setSub] = React.useState('0');
  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  const { doPost } = useDAGPostAPI({
    name,
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
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const onChangeFlowchart = (value: FlowchartType) => {
    setCookie('flowchart', value, { path: '/' });
    setFlowchart(value);
  };

  if (!run.status) {
    return null;
  }
  const handlers = getEventHandlers(run);

  return (
    <React.Fragment>
      <Box>
        <Stack direction="row" justifyContent="space-between">
          <SubTitle>Overview</SubTitle>
          <FlowchartSwitch value={flowchart} onChange={onChangeFlowchart} />
        </Stack>
        <BorderedBox
          sx={{
            mt: 2,
            py: 2,
            px: 2,
            display: 'flex',
            flexDirection: 'column',
            overflowX: 'auto',
          }}
        >
          <Tabs
            value={sub}
            onChange={(_, v) => setSub(v)}
            TabIndicatorProps={{
              style: {
                display: 'none',
              },
            }}
          >
            <Tab
              value="0"
              icon={<FontAwesomeIcon icon={faShareNodes} />}
              label="Graph"
              sx={{ minHeight: '40px', fontSize: '0.8rem' }}
            />
            <Tab
              value="1"
              icon={<FontAwesomeIcon icon={faChartGantt} />}
              label="Timeline"
              sx={{ minHeight: '40px', fontSize: '0.8rem' }}
            />
          </Tabs>

          <Box
            sx={{
              overflowX: 'auto',
            }}
          >
            {sub == '0' ? (
              <Graph
                steps={run.nodes}
                type="status"
                flowchart={flowchart}
                onClickNode={onSelectStepOnGraph}
                showIcons={run.status > Status.NotStarted}
                animate={run.status == Status.Running}
              ></Graph>
            ) : (
              <TimelineChart status={run}></TimelineChart>
            )}
          </Box>
        </BorderedBox>
      </Box>

      <Box>
        <DAGContext.Consumer>
          {(props) => (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <Box sx={{ mt: 2 }}>
                  <DAGStatusOverview
                    status={run}
                    {...props}
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
