/**
 * DAGGraph component provides a tabbed interface for visualizing DAG runs as either a graph or timeline.
 *
 * @module features/dags/components/visualization
 */
import React from 'react';
import { Graph, FlowchartType, TimelineChart } from './';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import BorderedBox from '../../../../ui/BorderedBox';
import { useCookies } from 'react-cookie';
import { FlowchartSwitch } from './';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChartGantt, faShareNodes } from '@fortawesome/free-solid-svg-icons';
import { components, Status } from '../../../../api/v2/schema';

/**
 * Props for the DAGGraph component
 */
type Props = {
  /** DAG run details containing execution information */
  run: components['schemas']['RunDetails'];
  /** Callback for when a step is selected in the graph */
  onSelectStep?: (id: string) => void;
};

/**
 * DAGGraph component provides a tabbed interface for visualizing DAG runs
 * with options to switch between graph and timeline views
 */
function DAGGraph({ run, onSelectStep }: Props) {
  // Active tab state (0 = Graph, 1 = Timeline)
  const [sub, setSub] = React.useState('0');

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

  /**
   * Handle flowchart direction change and save preference to cookie
   */
  const onChangeFlowchart = (value: FlowchartType) => {
    if (!value) {
      return;
    }
    setCookie('flowchart', value, { path: '/' });
    setFlowchart(value);
  };

  return (
    <Box>
      <Stack direction="row" justifyContent="start" my={2}>
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
            />
          ) : (
            <TimelineChart status={run} />
          )}
        </Box>
      </BorderedBox>
    </Box>
  );
}

export default DAGGraph;
