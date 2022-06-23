import React from "react";
import { DAGContext } from "../contexts/DAGContext";
import { DAG } from "../models/DAG";
import { Handlers, SchedulerStatus } from "../models/Status";
import Graph from "./Graph";
import NodeStatusTable from "./NodeStatusTable";
import StatusInfoTable from "./StatusInfoTable";
import Timeline from "./Timeline";
import { useWorkflowPostApi } from "../hooks/useWorkflowPostApi";
import StatusUpdateModal from "./StatusUpdateModal";
import { Step } from "../models/Step";
import { Box, Tab, Tabs, Paper } from "@mui/material";

type Props = {
  DAG: DAG;
  name: string;
  group: string;
  refresh: () => void;
};

function DAGStatus({ DAG, group, name, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [sub, setSub] = React.useState("0");
  const [selectedStep, setSelectedStep] = React.useState<Step | undefined>(
    undefined
  );
  const { doPost } = useWorkflowPostApi({
    name,
    group,
    onSuccess: refresh,
    requestId: DAG.Status?.RequestId,
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
      const status = DAG.Status?.Status;
      if (status == SchedulerStatus.Running || status == SchedulerStatus.None) {
        return;
      }
      // find the clicked step
      const n = DAG.Status?.Nodes.find(
        (n) => n.Step.Name.replace(/\s/g, "_") == id
      );
      if (n) {
        setSelectedStep(n.Step);
        setModal(true);
      }
    },
    [DAG]
  );
  if (!DAG.Status) {
    return null;
  }
  const handlers = Handlers(DAG.Status);
  return (
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
          sx={{
            overflowX: "auto",
          }}
        >
          {sub == "0" ? (
            <Graph
              steps={DAG.Status.Nodes}
              type="status"
              onClickNode={onSelectStepOnGraph}
            ></Graph>
          ) : (
            <Timeline status={DAG.Status}></Timeline>
          )}
        </Box>
      </Paper>

      <Box sx={{ mx: 4 }}>
        <DAGContext.Consumer>
          {(props) => (
            <React.Fragment>
              <Box sx={{ mt: 2 }}>
                <StatusInfoTable
                  status={DAG.Status}
                  {...props}
                ></StatusInfoTable>
              </Box>

              <Box sx={{ mt: 2 }}>
                <NodeStatusTable
                  nodes={DAG.Status!.Nodes}
                  status={DAG.Status!}
                  {...props}
                ></NodeStatusTable>
              </Box>

              <Box sx={{ mt: 2 }}>
                <NodeStatusTable
                  nodes={handlers}
                  status={DAG.Status!}
                  {...props}
                ></NodeStatusTable>
              </Box>
            </React.Fragment>
          )}
        </DAGContext.Consumer>
      </Box>

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </React.Fragment>
  );
}

export default DAGStatus;
