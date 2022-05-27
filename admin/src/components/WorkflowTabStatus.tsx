import React from "react";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { DAG } from "../models/Dag";
import { Handlers, SchedulerStatus } from "../models/Status";
import GraphDag from "./GraphDag";
import NodeTable from "./NodeTable";
import StatusInfo from "./StatusInfo";
import Timeline from "./GraphTimeline";
import { useWorkflowPostApi } from "../hooks/useWorkflowPostApi";
import StatusUpdateModal from "./StatusUpdateModal";
import { Step } from "../models/Step";

type Props = {
  workflow: DAG;
  subtab: number;
  name: string;
  group: string;
  refresh: () => void;
};

function WorkflowTabStatus({ workflow, subtab, group, name, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [selectedStep, setSelectedStep] = React.useState<Step | undefined>(
    undefined
  );
  const { doPost } = useWorkflowPostApi({
    name,
    group,
    onSuccess: refresh,
    requestId: workflow.Status?.RequestId,
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
      const status = workflow.Status?.Status;
      if (status == SchedulerStatus.Running || status == SchedulerStatus.None) {
        return;
      }
      // find the clicked step
      const n = workflow.Status?.Nodes.find(
        (n) => n.Step.Name.replace(/\s/g, "_") == id
      );
      if (n) {
        setSelectedStep(n.Step);
        setModal(true);
      }
    },
    [workflow]
  );
  if (!workflow.Status) {
    return null;
  }
  const handlers = Handlers(workflow.Status);
  return (
    <div>
      {subtab == 0 ? (
        <GraphDag
          steps={workflow.Status.Nodes}
          type="status"
          onClickNode={onSelectStepOnGraph}
        ></GraphDag>
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
            <NodeTable
              nodes={handlers}
              status={workflow.Status!}
              {...props}
            ></NodeTable>
          </React.Fragment>
        )}
      </WorkflowContext.Consumer>
      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </div>
  );
}

export default WorkflowTabStatus;
