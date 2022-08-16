import { Box } from '@mui/material';
import React from 'react';
import { GridData, LogData } from '../../models/api';
import { DAGContext } from '../../contexts/DAGContext';
import { Handlers, StatusFile } from '../../models';
import NodeStatusTable from '../molecules/NodeStatusTable';
import DAGStatusOverview from '../molecules/DAGStatusOverview';
import SubTitle from '../atoms/SubTitle';
import LoadingIndicator from '../atoms/LoadingIndicator';
import HistoryTable from '../molecules/HistoryTable';

type Props = {
  logData: LogData;
  isLoading: boolean;
};

function DAGHistory({ logData, isLoading }: Props) {
  if (!logData || logData.Logs?.length == 0 || logData.GridData?.length == 0) {
    if (isLoading) {
      return <LoadingIndicator />;
    }
    return <Box>Execution history was not found.</Box>;
  }
  return <DAGHistoryTable Logs={logData.Logs} GridData={logData.GridData} />;
}

type HistoryTableProps = {
  GridData: GridData[];
  Logs: StatusFile[];
};

function DAGHistoryTable({ GridData, Logs }: HistoryTableProps) {
  const [idx, setIdx] = React.useState(Logs.length - 1);
  const logs = React.useMemo(() => {
    return Logs;
  }, [Logs]);

  const handlers = logs.length > idx ? Handlers(logs[idx].Status) : null;

  return (
    <DAGContext.Consumer>
      {(props) => (
        <React.Fragment>
          <Box>
            <SubTitle>Execution History</SubTitle>
            <HistoryTable
              logs={logs}
              gridData={GridData}
              onSelect={setIdx}
              idx={idx}
            />
          </Box>

          {logs && logs[idx] ? (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Status</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <DAGStatusOverview
                    status={logs[idx].Status}
                    file={logs[idx].File}
                    {...props}
                  />
                </Box>
              </Box>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Steps</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={logs[idx].Status.Nodes}
                    status={logs[idx].Status}
                    file={logs[idx].File}
                    {...props}
                  />
                </Box>
              </Box>

              {handlers && handlers.length ? (
                <Box sx={{ mt: 3 }}>
                  <SubTitle>Lifecycle Hooks</SubTitle>
                  <Box sx={{ mt: 2 }}>
                    <NodeStatusTable
                      nodes={Handlers(logs[idx].Status)}
                      file={logs[idx].File}
                      status={logs[idx].Status}
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

export default DAGHistory;
