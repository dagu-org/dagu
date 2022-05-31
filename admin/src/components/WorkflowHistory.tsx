import { Box, Paper } from "@mui/material";
import React from "react";
import { LogData } from "../api/Workflow";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { Handlers } from "../models/Status";
import NodeStatusTable from "./NodeStatusTable";
import StatusHistTable from "./StatusHistTable";
import StatusInfoTable from "./StatusInfoTable";

type Props = {
  logData: LogData;
};

function WorkflowHistory({ logData }: Props) {
  const [idx, setIdx] = React.useState(logData.Logs.length - 1);
  const [logs, gridData] = React.useMemo(() => {
    return [logData.Logs.reverse(), logData.GridData];
  }, [logData]);
  return (
    <WorkflowContext.Consumer>
      {(props) => (
        <React.Fragment>
          <Paper
            sx={{
              pb: 4,
              px: 2,
              mx: 4,
              display: "flex",
              flexDirection: "column",
              overflowX: "auto",
              borderTopLeftRadius: 0,
              borderTopRightRadius: 0,
            }}
          >
            <StatusHistTable
              logs={logs}
              gridData={gridData}
              onSelect={setIdx}
              idx={idx}
            />
          </Paper>

          <Box sx={{ mx: 4 }}>
            {logs && logs[idx] ? (
              <React.Fragment>
                <Box sx={{ mt: 2 }}>
                  <StatusInfoTable status={logs[idx].Status} {...props} />
                </Box>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={logs[idx].Status.Nodes}
                    status={logs[idx].Status}
                    file={logs[idx].File}
                    {...props}
                  />
                </Box>
                <Box sx={{ mt: 2 }}>
                  <NodeStatusTable
                    nodes={Handlers(logs[idx].Status)}
                    file={logs[idx].File}
                    status={logs[idx].Status}
                    {...props}
                  />
                </Box>
              </React.Fragment>
            ) : null}
          </Box>
        </React.Fragment>
      )}
    </WorkflowContext.Consumer>
  );
}

export default WorkflowHistory;
