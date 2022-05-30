import React from "react";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { DAG } from "../models/Dag";
import { Handlers, SchedulerStatus } from "../models/Status";
import GraphDag from "./GraphDag";
import NodeStatusTable from "./NodeStatusTable";
import StatusInfoTable from "./StatusInfoTable";
import Timeline from "./GraphTimeline";
import { useWorkflowPostApi } from "../hooks/useWorkflowPostApi";
import StatusUpdateModal from "./StatusUpdateModal";
import { Step } from "../models/Step";
import { Box, Tab, Tabs, Paper } from "@mui/material";

type Props = {
  workflow: DAG;
  name: string;
  group: string;
  refresh: () => void;
  width: number;
};

function WorkflowStatus({ workflow, group, name, refresh, width }: Props) {
  const [modal, setModal] = React.useState(false);
  const [sub, setSub] = React.useState("0");
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
    <React.Fragment>
      <Paper
        sx={{
          pb: 4,
          px: 2,
          display: "flex",
          flexDirection: "column",
          overflowX: "auto",
          borderTopLeftRadius: 0,
          borderTopRightRadius: 0,
        }}
      >
        <Tabs
          value={sub}
          onChange={(_, v) => setSub(v)}
          TabIndicatorProps={{
            style: {
              display: "none",
            },
          }}
        >
          <Tab
            value="0"
            icon={<i className="fa-solid fa-share-nodes" />}
            label="Graph"
            sx={{ minHeight: "40px", fontSize: "0.8rem" }}
          />
          <Tab
            value="1"
            icon={<i className="fa-solid fa-chart-gantt" />}
            label="Timeline"
            sx={{ minHeight: "40px", fontSize: "0.8rem" }}
          />
        </Tabs>

        <Box
          maxWidth={width ? `${width - 100}px` : "100%"}
          sx={{
            overflowX: "auto",
          }}
        >
          {sub == "0" ? (
            <GraphDag
              steps={workflow.Status.Nodes}
              type="status"
              onClickNode={onSelectStepOnGraph}
            ></GraphDag>
          ) : (
            <Timeline status={workflow.Status}></Timeline>
          )}
        </Box>
      </Paper>

      <WorkflowContext.Consumer>
        {(props) => (
          <React.Fragment>
            <Box sx={{ mt: 2 }}>
              <StatusInfoTable
                status={workflow.Status}
                {...props}
              ></StatusInfoTable>
            </Box>

            <Box sx={{ mt: 2 }}>
              <NodeStatusTable
                nodes={workflow.Status!.Nodes}
                status={workflow.Status!}
                {...props}
              ></NodeStatusTable>
            </Box>

            <Box sx={{ mt: 2 }}>
              <NodeStatusTable
                nodes={handlers}
                status={workflow.Status!}
                {...props}
              ></NodeStatusTable>
            </Box>
          </React.Fragment>
        )}
      </WorkflowContext.Consumer>

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </React.Fragment>
  );
}

export default WorkflowStatus;
