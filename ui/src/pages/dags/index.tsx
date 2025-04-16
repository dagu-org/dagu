import React from 'react';
import DAGErrors from '../../features/dags/components/DAGErrors';
import Box from '@mui/material/Box';
import CreateDAGButton from '../../features/dags/components/CreateDAGButton';
import WithLoading from '../../ui/WithLoading';
import DAGTable from '../../features/dags/components/DAGTable';
import Title from '../../ui/Title';
import { useLocation } from 'react-router-dom';
import { AppBarContext } from '../../contexts/AppBarContext';
import DAGPagination from '../../features/dags/components/DAGPagination';
import { debounce } from 'lodash';
import { useUserPreferences } from '../../contexts/UserPreference';
import { useQuery } from '../../hooks/api';
import { components } from '../../api/v2/schema';

function DAGs() {
  const query = new URLSearchParams(useLocation().search);
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);
  const [searchText, setSearchText] = React.useState(query.get('search') || '');
  const [searchTag, setSearchTag] = React.useState(query.get('tag') || '');
  const [page, setPage] = React.useState(parseInt(query.get('page') || '1'));
  const [apiSearchText, setAPISearchText] = React.useState(
    query.get('search') || ''
  );
  const [apiSearchTag, setAPISearchTag] = React.useState(
    query.get('tag') || ''
  );
  const { preferences, updatePreference } = useUserPreferences();
  // Use preferences.pageLimit instead of local state
  const handlePageLimitChange = (newLimit: number) => {
    updatePreference('pageLimit', newLimit);
  };
  const { data, mutate, isLoading } = useQuery(
    '/dags',
    {
      params: {
        query: {
          page,
          perPage: preferences.pageLimit || 200,
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          name: apiSearchText ? apiSearchText : undefined,
          tag: apiSearchTag ? apiSearchTag : undefined,
        },
      },
    },
    {
      refreshInterval: 10000,
      revalidateIfStale: false,
    }
  );

  const addSearchParam = (key: string, value: string) => {
    const locationQuery = new URLSearchParams(window.location.search);
    locationQuery.set(key, value);
    window.history.pushState(
      {},
      '',
      `${window.location.pathname}?${locationQuery.toString()}`
    );
  };

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  React.useEffect(() => {
    appBarContext.setTitle('DAGs');
  }, [appBarContext]);

  const { dagFiles, errorCount } = React.useMemo(() => {
    const dagFiles: components['schemas']['DAGFile'][] = [];

    let errorCount = 0;
    if (data) {
      for (const val of data.dags) {
        if (!val.errors?.length) {
          dagFiles.push(val);
        } else {
          errorCount += 1;
        }
      }
    }
    return {
      dagFiles,
      errorCount,
    };
  }, [data]);

  const pageChange = (page: number) => {
    addSearchParam('page', page.toString());
    setPage(page);
  };

  const debouncedAPISearchText = React.useMemo(
    () =>
      debounce((searchText: string) => {
        setAPISearchText(searchText);
      }, 500),
    []
  );

  const debouncedAPISearchTag = React.useMemo(
    () =>
      debounce((searchTag: string) => {
        setAPISearchTag(searchTag);
      }, 500),
    []
  );

  const searchTextChange = (searchText: string) => {
    addSearchParam('search', searchText);
    setSearchText(searchText);
    setPage(1);
    debouncedAPISearchText(searchText);
  };

  const searchTagChange = (searchTag: string) => {
    addSearchParam('tag', searchTag);
    setSearchTag(searchTag);
    setPage(1);
    debouncedAPISearchTag(searchTag);
  };

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
        <WithLoading loaded={!isLoading}>
          {data && (
            <React.Fragment>
              <DAGErrors
                dags={data.dags || []}
                errors={data.errors || []}
                hasError={errorCount > 0 || data.errors?.length > 0}
              ></DAGErrors>
              <DAGTable
                dags={dagFiles}
                group={group}
                refreshFn={refreshFn}
                searchText={searchText}
                handleSearchTextChange={searchTextChange}
                searchTag={searchTag}
                handleSearchTagChange={searchTagChange}
              ></DAGTable>
              <DAGPagination
                totalPages={data.pagination.totalPages}
                page={page}
                pageChange={pageChange}
                onPageLimitChange={handlePageLimitChange}
                pageLimit={preferences.pageLimit}
              />
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Box>
  );
}
export default DAGs;
