import React, { useMemo } from 'react';
import { Link, useParams, useLocation } from 'react-router-dom';
import DAGStatus from '../../../components/organizations/DAGStatus';
import { DAGContext } from '../../../contexts/DAGContext';
import DAGSpec from '../../../components/organizations/DAGSpec';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import Title from '../../../components/atoms/Title';
import DAGActions from '../../../components/molecules/DAGActions';
import DAGEditButtons from '../../../components/molecules/DAGEditButtons';
import LoadingIndicator from '../../../components/atoms/LoadingIndicator';
import { AppBarContext } from '../../../contexts/AppBarContext';
import StatusChip from '../../../components/atoms/StatusChip';
import { CalendarToday, TimerSharp } from '@mui/icons-material';
import moment from 'moment-timezone';
import { RunDetailsContext } from '../../../contexts/DAGStatusContext';
import { useQuery } from '../../../hooks/api';
import { components, Status } from '../../../api/v2/schema';
import DAGExecutionHistory from '../../../components/organizations/DAGExecutionHistory';
import ExecutionLog from '../../../components/organizations/ExecutionLog';
import StepLog from '../../../components/organizations/StepLog';

type Params = {
  location: string;
  name: string;
  tab?: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const appBarContext = React.useContext(AppBarContext);
  const { pathname } = useLocation();
  const { data, isLoading, mutate } = useQuery(
    '/dags/{fileId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: params.location || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );
  const baseUrl = `/dags/${params.location}`;
  const [currentRun, setCurrentRun] = React.useState<
    components['schemas']['RunDetails'] | undefined
  >();
  const query = new URLSearchParams(window.location.search);
  const requestId =
    query.get('requestId') || data?.latestRun.requestId || 'latest';
  const stepName = query.get('step');

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate, params.location]);

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.dag?.name || '');
      setCurrentRun(data.latestRun);
    }
  }, [data, appBarContext]);

  const tab = useMemo(() => {
    return params.tab || 'status';
  }, [params]);

  const formatDuration = (startDate: string, endDate: string) => {
    if (!startDate || !endDate) return '--';
    const duration = moment.duration(moment(endDate).diff(moment(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
  };

  if (!params.location || isLoading || !data) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        location: params.location || '',
        name: data.dag?.name || '',
      }}
    >
      <RunDetailsContext.Provider
        value={{
          data: currentRun,
          setData: (status: components['schemas']['RunDetails']) => {
            setCurrentRun(status);
          },
        }}
      >
        <Stack
          sx={{
            width: '100%',
            direction: 'column',
          }}
        >
          <Box
            sx={{
              mx: 4,
              display: 'flex',
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <Title>{data.dag?.name}</Title>
            <RunDetailsContext.Consumer>
              {(status) => (
                <DAGActions
                  status={status.data}
                  dag={data.dag}
                  location={params.location!}
                  refresh={refreshFn}
                />
              )}
            </RunDetailsContext.Consumer>
          </Box>
          {data.latestRun.status != Status.NotStarted ? (
            <Stack
              direction="row"
              spacing={2}
              sx={{ mx: 4, alignItems: 'center' }}
            >
              {data.latestRun.status ? (
                <StatusChip status={data.latestRun.status}>
                  {data.latestRun.statusText || ''}
                </StatusChip>
              ) : null}

              <Stack
                direction="row"
                color={'text.secondary'}
                sx={{ alignItems: 'center', ml: 1 }}
              >
                <CalendarToday sx={{ mr: 0.5 }} />
                {data.latestRun?.finishedAt
                  ? moment(data.latestRun.finishedAt).format(
                      'MMM D, YYYY HH:mm:ss Z'
                    )
                  : '--'}
              </Stack>

              <Stack
                direction="row"
                color={'text.secondary'}
                sx={{ alignItems: 'center', ml: 1 }}
              >
                <TimerSharp sx={{ mr: 0.5 }} />
                {data.latestRun.finishedAt
                  ? formatDuration(
                      data.latestRun.startedAt,
                      data.latestRun.finishedAt
                    )
                  : data.latestRun.startedAt
                  ? formatDuration(
                      data.latestRun.startedAt,
                      moment().toISOString()
                    )
                  : '--'}
              </Stack>
            </Stack>
          ) : null}
          <Stack
            sx={{
              mx: 4,
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <Tabs value={`${pathname}`}>
              <LinkTab label="Status" value={`${baseUrl}`} />
              <LinkTab label="Spec" value={`${baseUrl}/spec`} />
              <LinkTab label="History" value={`${baseUrl}/history`} />
              {pathname == `${baseUrl}/log` ||
              pathname == `${baseUrl}/scheduler-log` ? (
                <Tab label="Log" value={pathname} />
              ) : null}
            </Tabs>
            {pathname == `${baseUrl}/spec` ? (
              <DAGEditButtons location={data.dag?.location || ''} />
            ) : null}
          </Stack>
          <Box sx={{ mx: 4, flex: 1 }}>
            {tab == 'status' ? (
              <DAGStatus
                run={data.latestRun}
                location={data.dag?.location || ''}
              />
            ) : null}
            {tab == 'spec' ? <DAGSpec location={params.location} /> : null}
            {tab == 'history' ? (
              <DAGExecutionHistory location={data.dag?.location || ''} />
            ) : null}
            {tab == 'scheduler-log' ? (
              <ExecutionLog name={data.dag?.name || ''} requestId={requestId} />
            ) : null}
            {tab == 'log' && stepName ? (
              <StepLog
                dagName={data.dag?.name || ''}
                requestId={requestId}
                stepName={stepName}
              />
            ) : null}
          </Box>
        </Stack>
      </RunDetailsContext.Provider>
    </DAGContext.Provider>
  );
}
export default DAGDetails;

interface LinkTabProps {
  label?: string;
  value: string;
}

function LinkTab({ value, ...props }: LinkTabProps) {
  return (
    <Link to={value}>
      <Tab value={value} {...props} />
    </Link>
  );
}
