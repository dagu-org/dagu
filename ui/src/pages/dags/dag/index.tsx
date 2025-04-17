import React, { useMemo } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { DAGStatus } from '../../../features/dags/components';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { DAGSpec } from '../../../features/dags/components/dag-editor';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import { LinkTab } from '../../../features/dags/components/common';
import { DAGEditButtons } from '../../../features/dags/components/dag-editor';
import LoadingIndicator from '../../../ui/LoadingIndicator';
import { AppBarContext } from '../../../contexts/AppBarContext';
import moment from 'moment-timezone';
import { RunDetailsContext } from '../../../features/dags/contexts/DAGStatusContext';
import { useQuery } from '../../../hooks/api';
import { components, Status } from '../../../api/v2/schema';
import {
  DAGExecutionHistory,
  ExecutionLog,
  StepLog,
} from '../../../features/dags/components/dag-execution';
import { DAGHeader } from '../../../features/dags/components/dag-details';

type Params = {
  fileId: string;
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
          fileId: params.fileId || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );
  const baseUrl = `/dags/${params.fileId}`;
  const [currentRun, setCurrentRun] = React.useState<
    components['schemas']['RunDetails'] | undefined
  >();
  const query = new URLSearchParams(window.location.search);
  const requestId =
    query.get('requestId') || data?.latestRun.requestId || 'latest';
  const stepName = query.get('step');

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate, params.fileId]);

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

  if (!params.fileId || isLoading || !data) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileId: params.fileId || '',
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
          <DAGHeader
            dag={data.dag}
            latestRun={data.latestRun}
            fileId={params.fileId || ''}
            refreshFn={refreshFn}
            formatDuration={formatDuration}
          />
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
              <DAGEditButtons fileId={params.fileId || ''} />
            ) : null}
          </Stack>
          <Box sx={{ mx: 4, flex: 1 }}>
            {tab == 'status' ? (
              <DAGStatus run={data.latestRun} fileId={params.fileId || ''} />
            ) : null}
            {tab == 'spec' ? <DAGSpec fileId={params.fileId} /> : null}
            {tab == 'history' ? (
              <DAGExecutionHistory fileId={params.fileId || ''} />
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
