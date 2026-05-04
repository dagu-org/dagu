// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { useLocation } from 'react-router-dom';
import {
  components,
  PathsDagsGetParametersQueryOrder,
  PathsDagsGetParametersQuerySort,
} from '../../api/v1/schema';
import { Button } from '@/components/ui/button';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import { DAGDetailsModal } from '../../features/dags/components/dag-details';
import { DAGErrors } from '../../features/dags/components/dag-editor';
import { DAGTable } from '../../features/dags/components/dag-list';
import DAGListHeader from '../../features/dags/components/dag-list/DAGListHeader';
import { useClient, useQuery } from '../../hooks/api';
import { useDAGsListSSE } from '../../hooks/useDAGsListSSE';
import {
  sseFallbackOptions,
  useSSECacheSync,
} from '../../hooks/useSSECacheSync';
import {
  withoutWorkspaceLabels,
  workspaceSelectionKey,
  workspaceSelectionQuery,
} from '../../lib/workspace';
import LoadingIndicator from '@/components/ui/loading-indicator';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';

type DAGDefinitionsFilters = {
  searchText: string;
  searchLabels: string[];
  sortField: string;
  sortOrder: string;
};

type DAGsPageResponse = {
  dags: components['schemas']['DAGFile'][];
  errors: string[];
  pagination: components['schemas']['Pagination'];
};

const areLabelsEqual = (a: string[], b: string[]): boolean => {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((label, i) => label === sortedB[i]);
};

const areDAGDefinitionsFiltersEqual = (
  a: DAGDefinitionsFilters,
  b: DAGDefinitionsFilters
) =>
  a.searchText === b.searchText &&
  areLabelsEqual(a.searchLabels, b.searchLabels) &&
  a.sortField === b.sortField &&
  a.sortOrder === b.sortOrder;

function mergeUniqueDAGFiles(
  head: components['schemas']['DAGFile'][],
  older: components['schemas']['DAGFile'][]
): components['schemas']['DAGFile'][] {
  const merged: components['schemas']['DAGFile'][] = [];
  const seen = new Set<string>();

  for (const dag of [...head, ...older]) {
    if (seen.has(dag.fileName)) {
      continue;
    }
    seen.add(dag.fileName);
    merged.push(dag);
  }

  return merged;
}

function getNextPage(
  pagination: components['schemas']['Pagination'] | undefined
): number | null {
  if (!pagination) {
    return null;
  }

  if (
    pagination.nextPage > pagination.currentPage &&
    pagination.nextPage <= pagination.totalPages
  ) {
    return pagination.nextPage;
  }

  if (pagination.currentPage < pagination.totalPages) {
    return pagination.currentPage + 1;
  }

  return null;
}

function getDAGListQueryKey(query: Record<string, unknown>): string {
  return JSON.stringify(
    Object.entries(query)
      .filter(([, value]) => value !== undefined)
      .sort(([left], [right]) => left.localeCompare(right))
  );
}

function useAutoLoadMore(
  sentinelRef: React.RefObject<HTMLDivElement | null>,
  enabled: boolean,
  onLoadMore: () => void
) {
  React.useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !enabled || typeof IntersectionObserver === 'undefined') {
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          onLoadMore();
        }
      },
      { threshold: 0.1 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [enabled, onLoadMore, sentinelRef]);
}

function supportsIntersectionObserver(): boolean {
  return typeof IntersectionObserver !== 'undefined';
}

function DAGsContent() {
  const location = useLocation();
  const query = React.useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );
  const group = query.get('group') || '';
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const client = useClient();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const workspaceSelection = appBarContext.workspaceSelection;
  const workspaceQuery = React.useMemo(
    () => workspaceSelectionQuery(workspaceSelection),
    [workspaceSelection]
  );
  const workspaceKey = workspaceSelectionKey(workspaceSelection);
  const searchStateScope = JSON.stringify({
    remoteNode,
    workspace: workspaceKey,
  });
  const { preferences } = useUserPreferences();
  const previousWorkspaceKeyRef = React.useRef(workspaceKey);
  const [selectedDAG, setSelectedDAG] = React.useState<string | null>(null);
  const [olderDAGFiles, setOlderDAGFiles] = React.useState<
    components['schemas']['DAGFile'][]
  >([]);
  const [continuationPageOverride, setContinuationPageOverride] =
    React.useState<number | null | undefined>(undefined);
  const [isLoadingMore, setIsLoadingMore] = React.useState(false);
  const [loadMoreError, setLoadMoreError] = React.useState<string | null>(null);
  const loadMoreSentinelRef = React.useRef<HTMLDivElement>(null);
  const autoLoadPendingRef = React.useRef(false);
  const loadMoreControllerRef = React.useRef<AbortController | null>(null);
  const paginationGenerationRef = React.useRef(0);

  const defaultFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText: '',
      searchLabels: [],
      sortField: 'name',
      sortOrder: 'asc',
    }),
    []
  );

  const [searchText, setSearchText] = React.useState(defaultFilters.searchText);
  const [searchLabels, setSearchLabels] = React.useState<string[]>(
    defaultFilters.searchLabels
  );
  const [sortField, setSortField] = React.useState(defaultFilters.sortField);
  const [sortOrder, setSortOrder] = React.useState(defaultFilters.sortOrder);
  const debouncedSearchText = useDebouncedValue(searchText, 500);
  const debouncedSearchLabels = useDebouncedValue(searchLabels, 500);

  React.useEffect(() => {
    if (previousWorkspaceKeyRef.current === workspaceKey) {
      return;
    }
    previousWorkspaceKeyRef.current = workspaceKey;
    setSelectedDAG(null);
  }, [workspaceKey]);

  const resetLoadedPages = React.useCallback(() => {
    paginationGenerationRef.current += 1;
    loadMoreControllerRef.current?.abort();
    loadMoreControllerRef.current = null;
    setOlderDAGFiles([]);
    setContinuationPageOverride(undefined);
    setLoadMoreError(null);
    setIsLoadingMore(false);
  }, []);

  const currentFilters = React.useMemo<DAGDefinitionsFilters>(
    () => ({
      searchText,
      searchLabels,
      sortField,
      sortOrder,
    }),
    [searchText, searchLabels, sortField, sortOrder]
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
      searchStateScope
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

    if (params.has('labels') || params.has('tags')) {
      const labelsParam = params.get('labels') ?? params.get('tags') ?? '';
      urlFilters.searchLabels = labelsParam
        ? labelsParam
            .split(',')
            .map((t) => t.trim().toLowerCase())
            .filter((t) => t !== '')
            .filter((t) => withoutWorkspaceLabels([t]).length > 0)
        : [];
      hasUrlFilters = true;
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
        searchState.writeState('dagDefinitions', searchStateScope, next);
      }
      return;
    }

    setSearchText(next.searchText);
    setSearchLabels(next.searchLabels);
    setSortField(next.sortField);
    setSortOrder(next.sortOrder);

    lastPersistedFiltersRef.current = next;
    searchState.writeState('dagDefinitions', searchStateScope, next);
  }, [defaultFilters, location.search, searchState, searchStateScope]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areDAGDefinitionsFiltersEqual(persisted, currentFilters)) {
      return;
    }

    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('dagDefinitions', searchStateScope, currentFilters);
  }, [currentFilters, searchState, searchStateScope]);

  const queryParams = React.useMemo(
    () => ({
      remoteNode,
      page: 1,
      perPage: preferences.pageLimit || 200,
      name: debouncedSearchText || undefined,
      labels:
        debouncedSearchLabels.length > 0
          ? debouncedSearchLabels.join(',')
          : undefined,
      sort: sortField,
      order: sortOrder,
      ...workspaceQuery,
    }),
    [
      remoteNode,
      preferences.pageLimit,
      debouncedSearchText,
      debouncedSearchLabels,
      sortField,
      sortOrder,
      workspaceQuery,
    ]
  );
  const queryKey = React.useMemo(
    () => getDAGListQueryKey(queryParams),
    [queryParams]
  );

  const dagsListSSE = useDAGsListSSE(queryParams);
  const { data, mutate, isLoading } = useQuery(
    '/dags',
    {
      params: {
        query: {
          ...queryParams,
          sort: sortField as PathsDagsGetParametersQuerySort,
          order: sortOrder as PathsDagsGetParametersQueryOrder,
        },
      },
    },
    {
      ...sseFallbackOptions(dagsListSSE),
      keepPreviousData: true,
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  );
  useSSECacheSync(dagsListSSE, mutate);

  React.useEffect(() => {
    resetLoadedPages();
  }, [queryKey, resetLoadedPages]);

  const addSearchParam = (key: string, value: string | string[]) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (key === 'labels') {
      locationQuery.delete('tags');
    }
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
    resetLoadedPages();
    setTimeout(() => mutate(), 500);
  }, [mutate, resetLoadedPages]);

  const handleSelectDAG = React.useCallback((fileName: string) => {
    setSelectedDAG(fileName);
  }, []);

  React.useEffect(() => {
    appBarContext.setTitle('Workflows');
  }, [appBarContext]);

  const searchTextChange = (searchText: string) => {
    addSearchParam('search', searchText);
    setSearchText(searchText);
  };

  const searchLabelsChange = (labels: string[]) => {
    addSearchParam('labels', labels);
    setSearchLabels(labels);
  };

  const handleSortChange = (field: string, order: string) => {
    addSearchParam('sort', field);
    addSearchParam('order', order);
    setSortField(field);
    setSortOrder(order);
  };

  const nextPage =
    continuationPageOverride === undefined
      ? getNextPage(data?.pagination)
      : continuationPageOverride;
  const hasMore = nextPage !== null;
  const { dagFiles, errorCount } = React.useMemo(() => {
    const dags = data?.dags ?? [];
    const mergedDags = mergeUniqueDAGFiles(dags, olderDAGFiles);
    return {
      dagFiles: mergedDags,
      errorCount: mergedDags.filter((dag) => dag.errors?.length).length,
    };
  }, [data?.dags, olderDAGFiles]);

  const handleLoadMore = React.useCallback(async (): Promise<void> => {
    if (isLoadingMore || !nextPage) {
      return;
    }

    const generation = paginationGenerationRef.current;
    loadMoreControllerRef.current?.abort();
    const controller = new AbortController();
    loadMoreControllerRef.current = controller;
    setIsLoadingMore(true);
    setLoadMoreError(null);

    try {
      const response = await client.GET('/dags', {
        params: {
          query: {
            ...queryParams,
            page: nextPage,
            sort: sortField as PathsDagsGetParametersQuerySort,
            order: sortOrder as PathsDagsGetParametersQueryOrder,
          },
        },
        signal: controller.signal,
      });

      if (
        controller.signal.aborted ||
        generation !== paginationGenerationRef.current
      ) {
        return;
      }

      if (response.error) {
        const message =
          response.error &&
          typeof response.error === 'object' &&
          'message' in response.error
            ? String(response.error.message)
            : 'Failed to load more workflows';
        setLoadMoreError(message);
        return;
      }

      const pageData = (response.data ?? {
        dags: [],
        errors: [],
        pagination: {
          totalRecords: 0,
          currentPage: nextPage,
          totalPages: nextPage,
          nextPage: 0,
          prevPage: nextPage - 1,
        },
      }) as DAGsPageResponse;
      setOlderDAGFiles((previous) =>
        mergeUniqueDAGFiles(previous, pageData.dags ?? [])
      );
      setContinuationPageOverride(getNextPage(pageData.pagination));
    } catch (caughtError) {
      if (controller.signal.aborted) {
        return;
      }
      setLoadMoreError(
        caughtError instanceof Error
          ? caughtError.message
          : 'Failed to load more workflows'
      );
    } finally {
      if (loadMoreControllerRef.current === controller) {
        loadMoreControllerRef.current = null;
      }
      if (generation === paginationGenerationRef.current) {
        setIsLoadingMore(false);
      }
    }
  }, [client, isLoadingMore, nextPage, queryParams, sortField, sortOrder]);

  React.useEffect(() => {
    if (!isLoadingMore) {
      autoLoadPendingRef.current = false;
    }
  }, [isLoadingMore]);

  const canAutoLoadMore = supportsIntersectionObserver();
  useAutoLoadMore(
    loadMoreSentinelRef,
    canAutoLoadMore && hasMore && !isLoadingMore && !loadMoreError,
    () => {
      if (autoLoadPendingRef.current) {
        return;
      }
      autoLoadPendingRef.current = true;
      void handleLoadMore();
    }
  );

  return (
    <div className="max-w-7xl">
      <DAGListHeader onRefresh={refreshFn} />
      {data ? (
        <>
          <DAGErrors
            dags={dagFiles}
            errors={data.errors || []}
            hasError={(errorCount > 0 || data.errors?.length > 0) && !isLoading}
          />
          <DAGTable
            dags={dagFiles}
            group={group}
            refreshFn={refreshFn}
            searchText={searchText}
            handleSearchTextChange={searchTextChange}
            searchLabels={searchLabels}
            handleSearchLabelsChange={searchLabelsChange}
            isLoading={isLoading}
            sortField={sortField}
            sortOrder={sortOrder}
            onSortChange={handleSortChange}
            selectedDAG={selectedDAG}
            onSelectDAG={handleSelectDAG}
          />
          <div className="mt-3 flex flex-col items-center gap-2">
            {loadMoreError && (
              <div className="text-sm text-error">{loadMoreError}</div>
            )}
            {hasMore ? (
              <>
                <div ref={loadMoreSentinelRef} className="h-4 w-full" />
                {isLoadingMore ? (
                  <div className="text-sm text-muted-foreground">
                    Loading more workflows...
                  </div>
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => void handleLoadMore()}
                  >
                    {loadMoreError
                      ? 'Retry loading more'
                      : 'Load more workflows'}
                  </Button>
                )}
              </>
            ) : dagFiles.length > 0 ? (
              <div className="text-sm text-muted-foreground">
                All workflows are displayed.
              </div>
            ) : null}
          </div>
        </>
      ) : (
        <LoadingIndicator />
      )}

      {selectedDAG && (
        <DAGDetailsModal
          fileName={selectedDAG}
          isOpen={!!selectedDAG}
          onClose={() => setSelectedDAG(null)}
        />
      )}
    </div>
  );
}

function DAGs() {
  return <DAGsContent />;
}

export default DAGs;
