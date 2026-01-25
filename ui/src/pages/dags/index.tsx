import { debounce } from 'lodash';
import React from 'react';
import { useLocation } from 'react-router-dom';
import {
  components,
  PathsDagsGetParametersQueryOrder,
  PathsDagsGetParametersQuerySort,
} from '../../api/v2/schema';
import SplitLayout from '../../components/SplitLayout';
import TabBar from '../../components/TabBar';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { TabProvider, useTabContext } from '../../contexts/TabContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import { DAGDetailsPanel } from '../../features/dags/components/dag-details';
import { DAGErrors } from '../../features/dags/components/dag-editor';
import { DAGTable } from '../../features/dags/components/dag-list';
import DAGListHeader from '../../features/dags/components/dag-list/DAGListHeader';
import { useQuery } from '../../hooks/api';
import { useDAGsListSSE } from '../../hooks/useDAGsListSSE';
import LoadingIndicator from '../../ui/LoadingIndicator';

type DAGDefinitionsFilters = {
  searchText: string;
  searchTags: string[];
  page: number;
  sortField: string;
  sortOrder: string;
};

const areTagsEqual = (a: string[], b: string[]): boolean => {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((tag, i) => tag === sortedB[i]);
};

const areDAGDefinitionsFiltersEqual = (
  a: DAGDefinitionsFilters,
  b: DAGDefinitionsFilters
) =>
  a.searchText === b.searchText &&
  areTagsEqual(a.searchTags, b.searchTags) &&
  a.page === b.page &&
  a.sortField === b.sortField &&
  a.sortOrder === b.sortOrder;

function DAGsContent() {
  const location = useLocation();
  const query = React.useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const { preferences, updatePreference } = useUserPreferences();
  const { tabs, activeTabId, selectDAG, addTab, closeTab, getActiveFileName } =
    useTabContext();

  const defaultFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText: '',
      searchTags: [],
      page: 1,
      sortField: 'name',
      sortOrder: 'asc',
    }),
    []
  );

  const [searchText, setSearchText] = React.useState(defaultFilters.searchText);
  const [searchTags, setSearchTags] = React.useState<string[]>(
    defaultFilters.searchTags
  );
  const [page, setPage] = React.useState<number>(defaultFilters.page);
  const [apiSearchText, setAPISearchText] = React.useState(
    defaultFilters.searchText
  );
  const [apiSearchTags, setAPISearchTags] = React.useState<string[]>(
    defaultFilters.searchTags
  );
  const [sortField, setSortField] = React.useState(defaultFilters.sortField);
  const [sortOrder, setSortOrder] = React.useState(defaultFilters.sortOrder);

  // Get selected DAG from tab context
  const selectedDAG = getActiveFileName();

  const currentFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText,
      searchTags,
      page,
      sortField,
      sortOrder,
    }),
    [searchText, searchTags, page, sortField, sortOrder]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef = React.useRef<DAGDefinitionsFilters | null>(
    null
  );

  React.useEffect(() => {
    const params = new URLSearchParams(location.search);
    const stored = searchState.readState<DAGDefinitionsFilters>(
      'dagDefinitions',
      remoteNode
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

    if (params.has('tags')) {
      const tagsParam = params.get('tags') ?? '';
      urlFilters.searchTags = tagsParam
        ? tagsParam
            .split(',')
            .map((t) => t.trim().toLowerCase())
            .filter((t) => t !== '')
        : [];
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
        searchState.writeState('dagDefinitions', remoteNode, next);
      }
      return;
    }

    setSearchText(next.searchText);
    setSearchTags(next.searchTags);
    setPage(next.page);
    setAPISearchText(next.searchText);
    setAPISearchTags(next.searchTags);
    setSortField(next.sortField);
    setSortOrder(next.sortOrder);

    lastPersistedFiltersRef.current = next;
    searchState.writeState('dagDefinitions', remoteNode, next);
  }, [defaultFilters, location.search, remoteNode, searchState]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areDAGDefinitionsFiltersEqual(persisted, currentFilters)) {
      return;
    }

    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('dagDefinitions', remoteNode, currentFilters);
  }, [currentFilters, remoteNode, searchState]);

  const handlePageLimitChange = (newLimit: number) => {
    updatePreference('pageLimit', newLimit);
  };

  // SSE for real-time updates
  const sseResult = useDAGsListSSE(
    {
      page,
      perPage: preferences.pageLimit || 200,
      name: apiSearchText || undefined,
      tags: apiSearchTags.length > 0 ? apiSearchTags.join(',') : undefined,
      sort: sortField,
      order: sortOrder,
    },
    true
  );

  // Polling fallback (only when SSE fails or not connected)
  const usePolling = sseResult.shouldUseFallback || !sseResult.isConnected;

  const { data: pollingData, mutate, isLoading } = useQuery(
    '/dags',
    {
      params: {
        query: {
          page,
          perPage: preferences.pageLimit || 200,
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          name: apiSearchText ? apiSearchText : undefined,
          tags: apiSearchTags.length > 0 ? apiSearchTags.join(',') : undefined,
          sort: sortField as PathsDagsGetParametersQuerySort,
          order: sortOrder as PathsDagsGetParametersQueryOrder,
        },
      },
    },
    {
      refreshInterval: usePolling ? 2000 : 0,
      revalidateIfStale: false,
      keepPreviousData: true,
      isPaused: () => !usePolling,
    }
  );

  // Use SSE data when available, otherwise polling
  const data = sseResult.data || pollingData;

  const addSearchParam = (key: string, value: string | string[]) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (Array.isArray(value)) {
      if (value.length > 0) {
        locationQuery.set(key, value.join(','));
      } else {
        // Explicitly set to empty string to indicate empty list was processed
        // This is needed so that the URL sync logic knows to clear the state
        locationQuery.delete(key);
      }
    } else if (value && value.length > 0) {
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
        dagFiles.push(val);
        if (val.errors?.length) {
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

  const debouncedAPISearchTags = React.useMemo(
    () =>
      debounce((tags: string[]) => {
        setAPISearchTags(tags);
      }, 500),
    []
  );

  const searchTextChange = (searchText: string) => {
    addSearchParam('search', searchText);
    setSearchText(searchText);
    setPage(1);
    debouncedAPISearchText(searchText);
  };

  const searchTagsChange = (tags: string[]) => {
    addSearchParam('tags', tags);
    setSearchTags(tags);
    setPage(1);
    debouncedAPISearchTags(tags);
  };

  const handleSortChange = (field: string, order: string) => {
    addSearchParam('sort', field);
    addSearchParam('order', order);
    setSortField(field);
    setSortOrder(order);
    setPage(1);
  };

  const [lastValidData, setLastValidData] = React.useState<typeof data | null>(
    null
  );

  React.useEffect(() => {
    if (data) {
      setLastValidData(data);
    }
  }, [data]);

  const displayData = data || lastValidData;

  const leftPanel = (
    <div className="pr-2 pt-4 md:pt-6 lg:pt-8">
      <DAGListHeader onRefresh={refreshFn} />
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
            searchTags={searchTags}
            handleSearchTagsChange={searchTagsChange}
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
            selectedDAG={selectedDAG}
            onSelectDAG={selectDAG}
          />
        </>
      ) : (
        <LoadingIndicator />
      )}
    </div>
  );

  // Handle adding a new tab - creates an empty tab that will be filled on next DAG selection
  const handleAddTab = () => {
    // Find a DAG to open in the new tab (first one not already open)
    const openFileNames = new Set(tabs.map((t) => t.fileName));
    const availableDAG = dagFiles.find((d) => !openFileNames.has(d.fileName));
    if (availableDAG) {
      addTab(availableDAG.fileName, availableDAG.dag.name);
    }
  };

  // Handle closing the active tab
  const handleCloseActiveTab = () => {
    if (activeTabId) {
      closeTab(activeTabId);
    }
  };

  const rightPanel =
    tabs.length > 0 ? (
      <div className="flex flex-col h-full">
        <TabBar onAddTab={handleAddTab} />
        <div className="flex-1 overflow-hidden">
          {selectedDAG && (
            <DAGDetailsPanel
              fileName={selectedDAG}
              onClose={handleCloseActiveTab}
            />
          )}
        </div>
      </div>
    ) : null;

  return (
    <div className="-my-4 md:-my-6 lg:-my-8 h-[calc(100%+2rem)] md:h-[calc(100%+3rem)] lg:h-[calc(100%+4rem)]">
      <SplitLayout
        leftPanel={leftPanel}
        rightPanel={rightPanel}
        defaultLeftWidth={40}
        emptyRightMessage="Select a DAG to view details"
      />
    </div>
  );
}

// Wrap with TabProvider
function DAGs() {
  return (
    <TabProvider>
      <DAGsContent />
    </TabProvider>
  );
}

export default DAGs;
