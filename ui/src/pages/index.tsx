import React from 'react';
import { ListWorkflowsResponse } from '../models/api';
import { Box, Grid } from '@mui/material';
import { SchedulerStatus } from '../models';
import { statusColorMapping } from '../consts';
import DashboardMetric from '../components/molecules/DashboardMetric';
import DashboardTimechart from '../components/molecules/DashboardTimechart';
import Title from '../components/atoms/Title';
import { AppBarContext } from '../contexts/AppBarContext';
import useSWR from 'swr';

type metrics = Record<SchedulerStatus, number>;

const defaultMetrics: metrics = {} as metrics;
for (const value in SchedulerStatus) {
  if (!isNaN(Number(value))) {
    const status = Number(value) as SchedulerStatus;
    defaultMetrics[status] = 0;
  }
}

function Dashboard() {
  const [metrics, setMetrics] = React.useState<metrics>(defaultMetrics);
  const appBarContext = React.useContext(AppBarContext);
  const { data } = useSWR<ListWorkflowsResponse>(`/dags`, null, {
    refreshInterval: 10000,
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
    appBarContext.setTitle('Dashboard');
  }, [appBarContext]);

  return (
    <Grid container spacing={3} sx={{ mx: 2, width: '100%' }}>
      {(
        [
          [SchedulerStatus.Success, 'Successful'],
          [SchedulerStatus.Error, 'Failed'],
          [SchedulerStatus.Running, 'Running'],
          [SchedulerStatus.Cancel, 'Canceled'],
        ] as Array<[SchedulerStatus, string]>
      ).map(([status, label]) => (
        <Grid item xs={12} md={4} lg={3} key={label}>
          <Box
            sx={{
              px: 2,
              display: 'flex',
              flexDirection: 'column',
              height: 240,
            }}
          >
            <DashboardMetric
              title={label}
              color={statusColorMapping[status].backgroundColor}
              value={metrics[status]}
            />
          </Box>
        </Grid>
      ))}

      <Grid item xs={12}>
        <Box
          sx={{
            p: 2,
            height: '100%',
          }}
        >
          <Title>Timeline</Title>
          <DashboardTimechart data={data?.DAGs || []} />
        </Box>
      </Grid>
    </Grid>
  );
}
export default Dashboard;
