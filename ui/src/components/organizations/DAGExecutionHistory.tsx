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
import { components } from '../../api/v2/schema';
import { useQuery } from '../../hooks/api';
import { AppBarContext } from '../../contexts/AppBarContext';

type Props = {
  location: string;
};

function DAGExecutionHistory({ location }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const { data } = useQuery(
    '/dags/{dagLocation}/runs',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          dagLocation: location,
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
  gridData: components['schemas']['DAGLogGridItem'][] | null;
  runs: components['schemas']['RunDetails'][] | null;
};

function DAGHistoryTable({ gridData, runs }: HistoryTableProps) {
  const [idx, setIdx] = React.useState(runs ? runs.length - 1 : 0);
  const dagStatusContext = React.useContext(RunDetailsContext);

  let handlers: components['schemas']['Node'][] | null = null;
  if (runs && idx < runs.length && runs[idx]) {
    handlers = getEventHandlers(runs[idx]);
  }
  const reversedRuns = useMemo(() => {
    return [...(runs || [])].reverse();
  }, [runs]);

  React.useEffect(() => {
    if (reversedRuns && reversedRuns[idx]) {
      dagStatusContext.setData(reversedRuns[idx]);
    }
  }, [reversedRuns, idx]);

  return (
    <DAGContext.Consumer>
      {(props) => (
        <React.Fragment>
          <Box>
            <SubTitle>Execution History</SubTitle>
            <HistoryTable
              runs={reversedRuns || []}
              gridData={gridData || []}
              onSelect={setIdx}
              idx={idx}
            />
          </Box>

          {reversedRuns && reversedRuns[idx] ? (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Status</SubTitle>
                <Box sx={{ mt: 2 }}>
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
        </React.Fragment>
      )}
    </DAGContext.Consumer>
  );
}

export default DAGExecutionHistory;
