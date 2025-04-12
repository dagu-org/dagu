import React from 'react';
import { Box, Grid } from '@mui/material';
import { SchedulerStatus } from '../models';
import { statusColorMapping } from '../consts';
import DashboardMetric from '../components/molecules/DashboardMetric';
import DashboardTimeChart from '../components/molecules/DashboardTimechart';
import Title from '../components/atoms/Title';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { useQuery } from '../hooks/api';

type metrics = Record<SchedulerStatus, number>;

const defaultMetrics: metrics = {} as metrics;
for (const value in SchedulerStatus) {
  if (!isNaN(Number(value))) {
    const status = Number(value) as SchedulerStatus;
    defaultMetrics[status] = 0;
  }
}

function Dashboard() {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const queryParams = {
    perPage: config.maxDashboardPageLimit || 200,
    remoteNode: appBarContext.selectedRemoteNode || 'local',
  };
  if (appBarContext.selectedRemoteNode) {
    queryParams.remoteNode = appBarContext.selectedRemoteNode;
  }
  const { data } = useQuery('/dags', {
    params: { query: queryParams },
  });

  const metrics = { ...defaultMetrics };
  data?.dags.forEach((dag) => {
    metrics[dag.latestRun.status] += 1;
  });

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
              color={statusColorMapping[status]?.backgroundColor}
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
          <Title>{`Timeline in ${config.tz}`}</Title>
          <DashboardTimeChart data={data?.dags || []} />
        </Box>
      </Grid>
    </Grid>
  );
}
export default Dashboard;
