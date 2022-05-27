import React from "react";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { DAG } from "../models/Dag";
import { Handlers, Status } from "../models/Status";
import GraphDag from "./GraphDag";
import NodeTable from "./NodeTable";
import StatusInfo from "./StatusInfo";
import Timeline from "./GraphTimeline";

type Props = {
  workflow: DAG;
  subtab: number;
};

function WorkflowTabStatus({ workflow, subtab }: Props) {
  if (!workflow.Status) {
    return null;
  }
  const handlers = Handlers(workflow.Status);
  return (
    <div>
      {subtab == 0 ? (
        <GraphDag steps={workflow.Status.Nodes} type="status"></GraphDag>
      ) : (
        <Timeline status={workflow.Status}></Timeline>
      )}
      <WorkflowContext.Consumer>
        {(props) => (
          <React.Fragment>
            <StatusInfo status={workflow.Status} {...props}></StatusInfo>
            <NodeTable
              nodes={workflow.Status!.Nodes}
              status={workflow.Status!}
              {...props}
            ></NodeTable>
            <NodeTable nodes={handlers} status={workflow.Status!} {...props}></NodeTable>
          </React.Fragment>
        )}
      </WorkflowContext.Consumer>
    </div>
  );
}

export default WorkflowTabStatus;
