import React from "react";
import { LogData } from "../api/Workflow";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { Handlers } from "../models/Status";
import NodeTable from "./NodeTable";
import StatusHistTable from "./StatusHistTable";
import StatusInfo from "./StatusInfo";

type Props = {
  logData: LogData;
};

function WorkflowTabHist({ logData }: Props) {
  const [idx, setIdx] = React.useState(logData.Logs.length - 1);
  const [logs, gridData] = React.useMemo(() => {
    return [logData.Logs.reverse(), logData.GridData];
  }, [logData]);
  return (
    <WorkflowContext.Consumer>
      {(props) => (
        <div>
          <StatusHistTable
            logs={logs}
            gridData={gridData}
            onSelect={setIdx}
            idx={idx}
          />
          {logs && logs[idx] ? (
            <React.Fragment>
              <StatusInfo status={logs[idx].Status} {...props}></StatusInfo>
              <NodeTable
                nodes={logs[idx].Status.Nodes}
                status={logs[idx].Status}
                file={logs[idx].File}
                {...props}
              ></NodeTable>
              <NodeTable
                nodes={Handlers(logs[idx].Status)}
                file={logs[idx].File}
                status={logs[idx].Status}
                {...props}
              ></NodeTable>
            </React.Fragment>
          ) : null}
        </div>
      )}
    </WorkflowContext.Consumer>
  );
}

export default WorkflowTabHist;
