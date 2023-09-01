import React from 'react';
import DAGErrors from '../../components/molecules/DAGErrors';
import Box from '@mui/material/Box';
import CreateDAGButton from '../../components/molecules/CreateDAGButton';
import WithLoading from '../../components/atoms/WithLoading';
import DAGTable from '../../components/molecules/DAGTable';
import Title from '../../components/atoms/Title';
import { DAGItem, DAGDataType } from '../../models';
import { useLocation } from 'react-router-dom';
import { ListWorkflowsResponse } from '../../models/api';
import { AppBarContext } from '../../contexts/AppBarContext';
import useSWR, { useSWRConfig } from 'swr';

function DAGs() {
  const useQuery = () => new URLSearchParams(useLocation().search);
  const query = useQuery();
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);

  const { cache, mutate } = useSWRConfig();
  const { data } = useSWR<ListWorkflowsResponse>(`/dags`, null, {
    refreshInterval: 10000,
  });

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(`/dags`), 500);
  }, [mutate, cache]);

  React.useEffect(() => {
    appBarContext.setTitle('DAGs');
  }, [appBarContext]);

  const merged = React.useMemo(() => {
    const ret: DAGItem[] = [];
    if (data) {
      for (const val of data.DAGs) {
        if (!val.ErrorT) {
          ret.push({
            Type: DAGDataType.DAG,
            Name: val.DAG.Name,
            DAGStatus: val,
          });
        }
      }
    }
    return ret;
  }, [data]);

  return (
    <Box
      sx={{
        px: 2,
        mx: 4,
        display: 'flex',
        flexDirection: 'column',
        width: '100%',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Title>DAGs</Title>
        <CreateDAGButton />
      </Box>
      <Box>
        <WithLoading loaded={!!data && !!merged}>
          {data && (
            <React.Fragment>
              <DAGErrors
                DAGs={data.DAGs}
                errors={data.Errors}
                hasError={data.HasError}
              ></DAGErrors>
              <DAGTable
                DAGs={merged}
                group={group}
                refreshFn={refreshFn}
              ></DAGTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Box>
  );
}
export default DAGs;
