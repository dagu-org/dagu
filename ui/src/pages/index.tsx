import React from 'react';
import { Box } from '@mui/material';
import Grid from '@mui/material/Grid';
import { statusColorMapping } from '../consts';
import DashboardMetric from '../features/dashboard/components/DashboardMetric';
import DashboardTimeChart from '../features/dashboard/components/DashboardTimechart';
import Title from '../ui/Title';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { useQuery } from '../hooks/api';
import { Status } from '../api/v2/schema';

type metrics = Record<Status, number>;

const defaultMetrics: metrics = {} as metrics;
for (const value in Status) {
  if (!isNaN(Number(value))) {
    const status = Number(value) as Status;
    defaultMetrics[status] = 0;
  }
}

function Dashboard() {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const { data } = useQuery('/dags', {
    params: {
      query: {
        perPage: config.maxDashboardPageLimit || 200,
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
    },
  });

  const metrics = { ...defaultMetrics };
  data?.dags.forEach((dag) => {
    metrics[dag.latestRun.status]! += 1;
  });

  React.useEffect(() => {
    appBarContext.setTitle('Dashboard');
  }, [appBarContext]);

  return (
    <Grid container spacing={3} sx={{ mx: 2, width: '100%' }}>
      {(
        [
          [Status.Success, 'Successful'],
          [Status.Failed, 'Failed'],
          [Status.Running, 'Running'],
          [Status.Cancelled, 'Canceled'],
        ] as Array<[Status, string]>
      ).map(([status, label]) => (
        <Grid {...{ item: true, xs: 12, md: 4, lg: 3, key: label }}>
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

      <Grid {...{ item: true, xs: 12 }}>
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
