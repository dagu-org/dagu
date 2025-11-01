import { debounce } from 'lodash';
import React from 'react';
import { useLocation } from 'react-router-dom';
import {
  components,
  PathsDagsGetParametersQueryOrder,
  PathsDagsGetParametersQuerySort,
} from '../../api/v2/schema';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import { DAGErrors } from '../../features/dags/components/dag-editor';
import { DAGTable } from '../../features/dags/components/dag-list';
import DAGListHeader from '../../features/dags/components/dag-list/DAGListHeader';
import { useQuery } from '../../hooks/api';
import LoadingIndicator from '../../ui/LoadingIndicator';

type DAGDefinitionsFilters = {
  searchText: string;
  searchTag: string;
  page: number;
  sortField: string;
  sortOrder: string;
};

const areDAGDefinitionsFiltersEqual = (
  a: DAGDefinitionsFilters,
  b: DAGDefinitionsFilters
) =>
  a.searchText === b.searchText &&
  a.searchTag === b.searchTag &&
  a.page === b.page &&
  a.sortField === b.sortField &&
  a.sortOrder === b.sortOrder;

function DAGs() {
  const location = useLocation();
  const query = React.useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';
  const { preferences, updatePreference } = useUserPreferences();

  const defaultFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText: '',
      searchTag: '',
      page: 1,
      sortField: 'name',
      sortOrder: 'asc',
    }),
    []
  );

  const [searchText, setSearchText] = React.useState(
    defaultFilters.searchText
  );
  const [searchTag, setSearchTag] = React.useState(defaultFilters.searchTag);
  const [page, setPage] = React.useState<number>(defaultFilters.page);
  const [apiSearchText, setAPISearchText] = React.useState(
    defaultFilters.searchText
  );
  const [apiSearchTag, setAPISearchTag] = React.useState(
    defaultFilters.searchTag
  );
  const [sortField, setSortField] = React.useState(defaultFilters.sortField);
  const [sortOrder, setSortOrder] = React.useState(defaultFilters.sortOrder);

  const currentFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText,
      searchTag,
      page,
      sortField,
      sortOrder,
    }),
    [searchText, searchTag, page, sortField, sortOrder]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef =
    React.useRef<DAGDefinitionsFilters | null>(null);

  React.useEffect(() => {
    const params = new URLSearchParams(location.search);
    const stored = searchState.readState<DAGDefinitionsFilters>(
      'dagDefinitions',
      remoteKey
    );
    const base: DAGDefinitionsFilters = {
      ...defaultFilters,
      ...(stored ?? {}),
    };

    const urlFilters: Partial<DAGDefinitionsFilters> = {};
    let hasUrlFilters = false;

    if (params.has('search')) {
      urlFilters.searchText = params.get('search') ?? '';
      hasUrlFilters = true;
    }

    if (params.has('tag')) {
      urlFilters.searchTag = params.get('tag') ?? '';
      hasUrlFilters = true;
    }

    if (params.has('page')) {
      const pageParam = Number.parseInt(params.get('page') || '', 10);
      if (!Number.isNaN(pageParam) && pageParam > 0) {
        urlFilters.page = pageParam;
        hasUrlFilters = true;
      }
    }

    if (params.has('sort')) {
      urlFilters.sortField = params.get('sort') || defaultFilters.sortField;
      hasUrlFilters = true;
    }

    if (params.has('order')) {
      urlFilters.sortOrder = params.get('order') || defaultFilters.sortOrder;
      hasUrlFilters = true;
    }

    const next = hasUrlFilters ? { ...base, ...urlFilters } : base;
    const current = currentFiltersRef.current;

    if (current && areDAGDefinitionsFiltersEqual(current, next)) {
      if (hasUrlFilters) {
        lastPersistedFiltersRef.current = next;
        searchState.writeState('dagDefinitions', remoteKey, next);
      }
      return;
    }

    setSearchText(next.searchText);
    setSearchTag(next.searchTag);
    setPage(next.page);
    setAPISearchText(next.searchText);
    setAPISearchTag(next.searchTag);
    setSortField(next.sortField);
    setSortOrder(next.sortOrder);

    lastPersistedFiltersRef.current = next;
    searchState.writeState('dagDefinitions', remoteKey, next);
  }, [
    defaultFilters,
    location.search,
    remoteKey,
    searchState,
  ]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areDAGDefinitionsFiltersEqual(persisted, currentFilters)) {
      return;
    }

    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('dagDefinitions', remoteKey, currentFilters);
  }, [currentFilters, remoteKey, searchState]);

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
          sort: sortField as PathsDagsGetParametersQuerySort,
          order: sortOrder as PathsDagsGetParametersQueryOrder,
        },
      },
    },
    {
      refreshInterval: 1000,
      revalidateIfStale: false,
      keepPreviousData: true, // Keep previous data while loading new data
    }
  );

  const addSearchParam = (key: string, value: string) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (value && value.length > 0) {
      locationQuery.set(key, value);
    } else {
      locationQuery.delete(key);
    }
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
    appBarContext.setTitle('DAG Definitions');
  }, [appBarContext]);

  const { dagFiles, errorCount } = React.useMemo(() => {
    const dagFiles: components['schemas']['DAGFile'][] = [];
    let errorCount = 0;
    if (data && data.dags) {
      for (const val of data.dags) {
        dagFiles.push(val); // Always add the DAG to the list
        if (val.errors?.length) {
          errorCount += 1; // Count DAGs with errors
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

  const handleSortChange = (field: string, order: string) => {
    addSearchParam('sort', field);
    addSearchParam('order', order);
    setSortField(field);
    setSortOrder(order);
    setPage(1); // Reset to first page when sorting changes
  };

  // Store the last valid data to prevent blank screens during loading
  const [lastValidData, setLastValidData] = React.useState<typeof data | null>(
    null
  );

  // Update lastValidData whenever we get new data
  React.useEffect(() => {
    if (data) {
      setLastValidData(data);
    }
  }, [data]);

  // Use the current data if available, otherwise use the last valid data
  const displayData = data || lastValidData;

  return (
    <div className="flex flex-col">
      <DAGListHeader onRefresh={refreshFn} />

      {/* Content */}
      <div className="w-full">
        {displayData ? (
          <>
            <DAGErrors
              dags={displayData.dags || []}
              errors={displayData.errors || []}
              hasError={
                (errorCount > 0 || displayData.errors?.length > 0) && !isLoading
              }
            />
            <DAGTable
              dags={isLoading ? (lastValidData ? dagFiles : []) : dagFiles}
              group={group}
              refreshFn={refreshFn}
              searchText={searchText}
              handleSearchTextChange={searchTextChange}
              searchTag={searchTag}
              handleSearchTagChange={searchTagChange}
              pagination={{
                totalPages: displayData.pagination.totalPages,
                page: page,
                pageChange: pageChange,
                onPageLimitChange: handlePageLimitChange,
                pageLimit: preferences.pageLimit,
              }}
              isLoading={isLoading}
              sortField={sortField}
              sortOrder={sortOrder}
              onSortChange={handleSortChange}
            />
          </>
        ) : (
          /* Show initial loading state if no data yet */
          <LoadingIndicator />
        )}
      </div>
    </div>
  );
}

export default DAGs;
