import React, { useMemo } from 'react';
import { Link, useParams, Routes, Route, useLocation } from 'react-router-dom';
import { GetDAGResponse } from '../../../models/api';
import DAGSpecErrors from '../../../components/molecules/DAGSpecErrors';
import DAGStatus from '../../../components/organizations/DAGStatus';
import { DAGContext } from '../../../contexts/DAGContext';
import DAGSpec from '../../../components/organizations/DAGSpec';
import ExecutionHistory from '../../../components/organizations/ExecutionHistory';
import ExecutionLog from '../../../components/organizations/ExecutionLog';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import Title from '../../../components/atoms/Title';
import DAGActions from '../../../components/molecules/DAGActions';
import DAGEditButtons from '../../../components/molecules/DAGEditButtons';
import LoadingIndicator from '../../../components/atoms/LoadingIndicator';
import { AppBarContext } from '../../../contexts/AppBarContext';
import useSWR, { useSWRConfig } from 'swr';

type Params = {
  name: string;
  tab?: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const appBarContext = React.useContext(AppBarContext);
  const { pathname } = useLocation();

  const baseUrl = useMemo(
    () => `/dags/${encodeURI(params.name!)}`,
    [params.name]
  );
  const { data, isValidating, mutate } = useSWR<GetDAGResponse>(
    `/dags/${params.name}?tab=${params.tab ?? ''}&${new URLSearchParams(
      window.location.search
    ).toString()}`,
    null,
    {
      refreshInterval: 2000,
    }
  );

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate, params.name]);

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.Title);
    }
  }, [data, appBarContext]);

  const tab = useMemo(() => {
    return params.tab || 'status';
  }, [params]);

  if (!params.name || !data || !data.DAG) {
    return <LoadingIndicator />;
  }

  const ctx = {
    data: data,
    refresh: refreshFn,
    name: params.name,
  };

  return (
    <DAGContext.Provider value={ctx}>
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
          <Title>{data.Title}</Title>
          <DAGActions
            status={data.DAG.Status}
            dag={data.DAG.DAG}
            name={params.name!}
            refresh={refreshFn}
            redirectTo={`${baseUrl}`}
          />
        </Box>

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
            <DAGEditButtons name={params.name} />
          ) : null}
        </Stack>

        <Box sx={{ mt: 2, mx: 4 }}>
          <DAGSpecErrors errors={data.Errors} />
        </Box>

        <Box sx={{ mx: 4, flex: 1 }}>
          {tab == 'status' ? (
            <DAGStatus DAG={data.DAG} name={params.name} refresh={refreshFn} />
          ) : null}
          {tab == 'spec' ? <DAGSpec data={data} /> : null}
          {tab == 'history' ? (
            <ExecutionHistory logData={data.LogData} isLoading={isValidating} />
          ) : null}
          {tab == 'scheduler-log' ? <ExecutionLog log={data.ScLog} /> : null}
          {tab == 'log' ? <ExecutionLog log={data.StepLog} /> : null}
        </Box>
      </Stack>
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
