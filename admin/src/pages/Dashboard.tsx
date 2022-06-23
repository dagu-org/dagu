import React from "react";
import { GetDAGsResponse } from "../api/DAGs";
import Paper from "@mui/material/Paper";
import { useDAGGetAPI } from "../hooks/useDAGGetAPI";
import { Grid } from "@mui/material";
import { SchedulerStatus } from "../models/Status";
import { statusColorMapping } from "../consts";
import Metrics from "../components/Metrics";
import DashboardTimechart from "../components/DashboardTimechart";
import Title from "../components/Title";

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

  const { data, doGet } = useDAGGetAPI<GetDAGsResponse>("/", {});

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
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, [group]);

  return (
    <Grid container spacing={3} sx={{ mx: 4, width: "100%" }}>
      {(
        [
          [SchedulerStatus.Success, "Successful DAGs"],
          [SchedulerStatus.Error, "Failed DAGs"],
          [SchedulerStatus.Running, "Running DAGs"],
          [SchedulerStatus.Cancel, "Canceled DAGs"],
        ] as Array<[SchedulerStatus, string]>
      ).map(([status, label]) => (
        <Grid item xs={12} md={4} lg={3} key={label}>
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

      {/* {data?.DAGs ? ( */}
      <Grid item xs={12}>
        <Paper
          sx={{
            p: 2,
            height: "100%",
          }}
        >
          <Title>Timeline</Title>
          <DashboardTimechart data={data?.DAGs || []} />
        </Paper>
      </Grid>
      {/* ) : null} */}
    </Grid>
  );
}
export default Dashboard;
