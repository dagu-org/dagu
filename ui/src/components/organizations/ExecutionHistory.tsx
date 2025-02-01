import { Box } from '@mui/material';
import React from 'react';
import { GridData, LogData } from '../../models/api';
import { DAGContext } from '../../contexts/DAGContext';
import { Handlers, Node, StatusFile } from '../../models';
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
  GridData: GridData[] | null;
  Logs: StatusFile[] | null;
};

function DAGHistoryTable({ GridData, Logs }: HistoryTableProps) {
  const [idx, setIdx] = React.useState(Logs ? Logs.length - 1 : 0);

  let handlers: Node[] | null = null;
  if (Logs && Logs.length > idx) {
    handlers = Handlers(Logs[idx].Status);
  }

  return (
    <DAGContext.Consumer>
      {(props) => (
        <React.Fragment>
          <Box>
            <SubTitle>Execution History</SubTitle>
            <HistoryTable
              logs={Logs || []}
              gridData={GridData || []}
              onSelect={setIdx}
              idx={idx}
            />
          </Box>

          {Logs && Logs[idx] ? (
            <React.Fragment>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Status</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <DAGStatusOverview
                    status={Logs[idx].Status}
                    file={Logs[idx].File}
                    {...props}
                  />
                </Box>
              </Box>
              <Box sx={{ mt: 3 }}>
                <SubTitle>Steps</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={Logs[idx].Status.Nodes}
                    status={Logs[idx].Status}
                    file={Logs[idx].File}
                    {...props}
                  />
                </Box>
              </Box>

              {handlers && handlers.length ? (
                <Box sx={{ mt: 3 }}>
                  <SubTitle>Lifecycle Hooks</SubTitle>
                  <Box sx={{ mt: 2 }}>
                    <NodeStatusTable
                      nodes={Handlers(Logs[idx].Status)}
                      file={Logs[idx].File}
                      status={Logs[idx].Status}
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
