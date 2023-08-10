import React from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStatus } from '../../models';
import { Handlers, SchedulerStatus } from '../../models';
import Graph, { FlowchartType } from '../molecules/Graph';
import NodeStatusTable from '../molecules/NodeStatusTable';
import DAGStatusOverview from '../molecules/DAGStatusOverview';
import TimelineChart from '../molecules/TimelineChart';
import { useDAGPostAPI } from '../../hooks/useDAGPostAPI';
import StatusUpdateModal from '../molecules/StatusUpdateModal';
import { Step } from '../../models';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import SubTitle from '../atoms/SubTitle';
import BorderedBox from '../atoms/BorderedBox';
import { useCookies } from 'react-cookie';
import FlowchartSwitch from '../molecules/FlowchartSwitch';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChartGantt, faShareNodes } from '@fortawesome/free-solid-svg-icons';

type Props = {
  DAG: DAGStatus;
  name: string;
  refresh: () => void;
};

function DAGStatus({ DAG, name, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [sub, setSub] = React.useState('0');
  const [selectedStep, setSelectedStep] = React.useState<Step | undefined>(
    undefined
  );
  const { doPost } = useDAGPostAPI({
    name,
    onSuccess: refresh,
    requestId: DAG.Status?.RequestId,
  });
  const dismissModal = React.useCallback(() => {
    setModal(false);
  }, [setModal]);
  const onUpdateStatus = React.useCallback(
    async (step: Step, action: string) => {
      doPost(action, step.Name);
      dismissModal();
    },
    [refresh, dismissModal]
  );
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const status = DAG.Status?.Status;
      if (status == SchedulerStatus.Running || status == SchedulerStatus.None) {
        return;
      }
      // find the clicked step
      const n = DAG.Status?.Nodes.find(
        (n) => n.Step.Name.replace(/\s/g, '_') == id
      );
      if (n) {
        setSelectedStep(n.Step);
        setModal(true);
      }
    },
    [DAG]
  );
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const onChangeFlowchart = React.useCallback(
    (value: FlowchartType) => {
      setCookie('flowchart', value, { path: '/' });
      setFlowchart(value);
    },
    [setCookie, flowchart, setFlowchart]
  );

  if (!DAG.Status) {
    return null;
  }
  const handlers = Handlers(DAG.Status);

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
                steps={DAG.Status.Nodes}
                type="status"
                flowchart={flowchart}
                onClickNode={onSelectStepOnGraph}
              ></Graph>
            ) : (
              <TimelineChart status={DAG.Status}></TimelineChart>
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
                    status={DAG.Status}
                    {...props}
                  ></DAGStatusOverview>
                </Box>
              </Box>

              <Box sx={{ mt: 3 }}>
                <SubTitle>Steps</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={DAG.Status!.Nodes}
                    status={DAG.Status!}
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
                      status={DAG.Status!}
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
