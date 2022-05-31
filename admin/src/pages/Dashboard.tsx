import React from "react";
import { GetListResponse } from "../api/List";
import WorkflowErrors from "../components/WorkflowErrors";
import Box from "@mui/material/Box";
import CreateWorkflowButton from "../components/CreateWorkflowButton";
import WithLoading from "../components/WithLoading";
import WorkflowTable from "../components/WorkflowTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useGetApi } from "../hooks/useWorkflowsGetApi";
import { Grid } from "@mui/material";
import { SchedulerStatus } from "../models/Status";
import { statusColorMapping } from "../consts";
import Metrics from "../components/Metrics";
import { Schedule } from "@mui/icons-material";

type metrics = Record<SchedulerStatus, number>;

const defaultMetrics: metrics = {} as metrics;
for (const value in SchedulerStatus) {
  if (!isNaN(Number(value))) {
    const status = Number(value) as SchedulerStatus;
    defaultMetrics[status] = 0;
  }
}

function Dashboard() {
  const [group] = React.useState<string>(
    new URLSearchParams(window.location.search).get("group") || ""
  );

  const [metrics, setMetrics] = React.useState<metrics>(defaultMetrics);

  const { data, doGet } = useGetApi<GetListResponse>("/", {
    group: group,
  });

  React.useEffect(() => {
    if (!data) {
      return;
    }
    const m = { ...defaultMetrics };
    data.DAGs.forEach((wf) => {
      if (wf.Status && wf.Status.Status) {
        const status = wf.Status.Status;
        m[status] += 1;
      }
    });
    setMetrics(m as metrics);
  }, [data]);

  React.useEffect(() => {
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, []);

  return (
    <Grid container spacing={3} sx={{ mx: 4, width: "100%" }}>
      {(
        [
          [SchedulerStatus.Success, "Successful Workflows"],
          [SchedulerStatus.Error, "Failed Workflows"],
          [SchedulerStatus.Running, "Running Workflows"],
          [SchedulerStatus.Cancel, "Canceled Workflows"],
        ] as Array<[SchedulerStatus, string]>
      ).map(([status, label]) => (
        <Grid item xs={12} md={4} lg={3}>
          <Paper
            sx={{
              p: 2,
              display: "flex",
              flexDirection: "column",
              height: 240,
            }}
          >
            <Metrics
              title={label}
              color={statusColorMapping[status].backgroundColor}
              value={metrics[status]}
            />
          </Paper>
        </Grid>
      ))}
    </Grid>
  );
}
export default Dashboard;
