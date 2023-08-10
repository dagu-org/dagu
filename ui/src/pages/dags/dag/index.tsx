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
};

function DAGDetails() {
  const params = useParams<Params>();
  const appBarContext = React.useContext(AppBarContext);
  const path = useLocation().pathname;
  const baseUrl = useMemo(
    () => `/dags/${encodeURI(params.name!)}`,
    [params.name]
  );
  const { data, isValidating } = useSWR<GetDAGResponse>(
    `${path}?${new URLSearchParams(window.location.search).toString()}`,
    null,
    {
      refreshInterval: 2000,
    }
  );
  const { mutate } = useSWRConfig();

  const refreshFn = React.useCallback(() => {
    mutate(`${baseUrl}/`);
  }, [mutate, baseUrl]);

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.Title);
    }
  }, [data, appBarContext]);

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
          <Tabs value={`${path}`}>
            <LinkTab label="Status" value={`${baseUrl}`} />
            <LinkTab label="Spec" value={`${baseUrl}/spec`} />
            <LinkTab label="History" value={`${baseUrl}/history`} />
            {path == `${baseUrl}/log` || path == `${baseUrl}/scheduler-log` ? (
              <Tab label="Log" value={path} />
            ) : null}
          </Tabs>
          {path == `${baseUrl}/spec` ? (
            <DAGEditButtons name={params.name} />
          ) : null}
        </Stack>

        <Box sx={{ mt: 2, mx: 4 }}>
          <DAGSpecErrors errors={data.Errors} />
        </Box>

        <Box sx={{ mx: 4, flex: 1 }}>
          <Routes>
            <Route
              index
              element={
                <DAGStatus
                  DAG={data.DAG}
                  name={params.name}
                  refresh={refreshFn}
                />
              }
            />
            <Route path={'/spec'} element={<DAGSpec data={data} />} />
            <Route
              path={'/history'}
              element={
                <ExecutionHistory
                  logData={data.LogData}
                  isLoading={isValidating}
                />
              }
            />
            <Route
              path={'/scheduler-log'}
              element={<ExecutionLog log={data.ScLog} />}
            />
            <Route
              path={'/log'}
              element={<ExecutionLog log={data.StepLog} />}
            />
          </Routes>
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
