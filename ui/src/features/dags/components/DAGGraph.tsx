import React from 'react';
import Graph, { FlowchartType } from './Graph';
import TimelineChart from './TimelineChart';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import SubTitle from '../../../ui/SubTitle';
import BorderedBox from '../../../ui/BorderedBox';
import { useCookies } from 'react-cookie';
import FlowchartSwitch from './FlowchartSwitch';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChartGantt, faShareNodes } from '@fortawesome/free-solid-svg-icons';
import { components, Status } from '../../../api/v2/schema';

type Props = {
  run: components['schemas']['RunDetails'];
  onSelectStep?: (id: string) => void;
};

function DAGGraph({ run, onSelectStep }: Props) {
  const [sub, setSub] = React.useState('0');
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const onChangeFlowchart = (value: FlowchartType) => {
    setCookie('flowchart', value, { path: '/' });
    setFlowchart(value);
  };

  return (
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
              onClickNode={onSelectStep}
              showIcons={run.status > Status.NotStarted}
              animate={run.status == Status.Running}
            ></Graph>
          ) : (
            <TimelineChart status={run}></TimelineChart>
          )}
        </Box>
      </BorderedBox>
    </Box>
  );
}

export default DAGGraph;
