import React from 'react';
import DAGErrors from '../components/DAGErrors';
import Box from '@mui/material/Box';
import DAGCreationButton from '../components/DAGCreationButton';
import WithLoading from '../components/WithLoading';
import DAGTable from '../components/DAGTable';
import Title from '../components/Title';
import { useDAGGetAPI } from '../hooks/useDAGGetAPI';
import { DAGItem, DAGDataType } from '../models/DAGData';
import { useLocation } from 'react-router-dom';
import { GetDAGsResponse } from '../api/DAGs';

function DAGList() {
  const useQuery = () => new URLSearchParams(useLocation().search);
  const query = useQuery();
  const group = query.get('group') || '';

  const { data, doGet } = useDAGGetAPI<GetDAGsResponse>('/', {});

  React.useEffect(() => {
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, []);

  const merged = React.useMemo(() => {
    const ret: DAGItem[] = [];
    if (data) {
      for (const val of data.DAGs) {
        if (!val.ErrorT) {
          ret.push({
            Type: DAGDataType.DAG,
            Name: val.Config.Name,
            DAG: val,
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
        <DAGCreationButton refresh={doGet}></DAGCreationButton>
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
                refreshFn={doGet}
              ></DAGTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Box>
  );
}
export default DAGList;
