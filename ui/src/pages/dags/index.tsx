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
import DAGPagination from '../../components/molecules/DAGPagination';
import { debounce } from 'lodash';

function DAGs() {
  const useQuery = () => new URLSearchParams(useLocation().search);
  const query = useQuery();
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);
  const [searchText, setSearchText] = React.useState(query.get('search') || '');
  const [searchTag, setSearchTag] = React.useState(query.get('tag') || '');
  const [page, setPage] = React.useState(parseInt(query.get('page') || '1'));
  const [apiSearchText, setAPISearchText] = React.useState(query.get('search') || '');
  const [apiSearchTag, setAPISearchTag] = React.useState(query.get('tag') || '');

  const { cache, mutate } = useSWRConfig();
  const endPoint =`/dags?${new URLSearchParams(
    {
      page: page.toString(),
      limit: '50',
      searchName: apiSearchText,
      searchTag: apiSearchTag,
    }
  ).toString()}`
  const { data } = useSWR<ListWorkflowsResponse>(endPoint, null, {
    refreshInterval: 10000,
    revalidateIfStale: false,
  });

  const addSearchParam = (key: string, value: string) => {
    const locationQuery = new URLSearchParams(window.location.search);
    locationQuery.set(key, value);
    window.history.pushState({}, '', `${window.location.pathname}?${locationQuery.toString()}`);
  }

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(endPoint), 500);
  }, [mutate, cache]);

  React.useEffect(() => {
    appBarContext.setTitle('DAGs');
  }, [appBarContext]);

  const merged = React.useMemo(() => {
    const ret: DAGItem[] = [];
    if (data && data.DAGs) {
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

  const pageChange = (page: number) => {
    addSearchParam('page', page.toString());
    setPage(page);
  };

  const debouncedAPISearchText = React.useMemo(() => debounce((searchText: string) => {
    setAPISearchText(searchText);
  }, 500), []);

  const debouncedAPISearchTag = React.useMemo(() => debounce((searchTag: string) => {
    setAPISearchTag(searchTag);
  }, 500), []);

  const searchTextChange = (searchText: string) => {
    addSearchParam('search', searchText);
    setSearchText(searchText);
    setPage(1);
    debouncedAPISearchText(searchText);
  }

  const searchTagChange = (searchTag: string) => {
    addSearchParam('tag', searchTag);
    setSearchTag(searchTag);
    setPage(1);
    debouncedAPISearchTag(searchTag);
  }

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
                DAGs={data.DAGs || []}
                errors={data.Errors || []}
                hasError={data.HasError}
              ></DAGErrors>
              <DAGTable
                DAGs={merged}
                group={group}
                refreshFn={refreshFn}
                searchText={searchText}
                handleSearchTextChange={searchTextChange}
                searchTag={searchTag}
                handleSearchTagChange={searchTagChange}
              ></DAGTable>
              <DAGPagination totalPages={data.PageCount} page={page} pageChange={pageChange} />
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Box>
  );
}
export default DAGs;
