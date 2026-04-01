import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DAGPagination } from '@/features/dags/components/common';
import { Search as SearchIcon } from 'lucide-react';
import React, { useEffect, useMemo, useRef } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import SearchResult from '../../features/search/components/SearchResult';
import { useQuery } from '../../hooks/api';
import Title from '../../ui/Title';

const SEARCH_PAGE_SIZE = 10;

type SearchFilters = {
  searchVal: string;
  dagPage: number;
  docPage: number;
};

function parsePage(value: string | null): number {
  const parsed = Number.parseInt(value || '', 10);
  return Number.isNaN(parsed) || parsed < 1 ? 1 : parsed;
}

function buildSearchParams(filters: SearchFilters): URLSearchParams {
  const params = new URLSearchParams();
  const query = filters.searchVal.trim();

  if (!query) {
    return params;
  }

  params.set('q', query);
  if (filters.dagPage > 1) {
    params.set('dagPage', String(filters.dagPage));
  }
  if (filters.docPage > 1) {
    params.set('docPage', String(filters.docPage));
  }

  return params;
}

function Search() {
  const [, setSearchParams] = useSearchParams();
  const location = useLocation();
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );

  const currentFilters = useMemo<SearchFilters>(
    () => ({
      searchVal: queryParams.get('q') || '',
      dagPage: parsePage(queryParams.get('dagPage')),
      docPage: parsePage(queryParams.get('docPage')),
    }),
    [queryParams]
  );

  const [searchVal, setSearchVal] = React.useState(currentFilters.searchVal);

  useEffect(() => {
    setSearchVal(currentFilters.searchVal);
  }, [currentFilters.searchVal]);

  useEffect(() => {
    const hasUrlQuery = queryParams.has('q');
    const stored = searchState.readState<SearchFilters>('searchPage', remoteKey);

    if (!hasUrlQuery && stored?.searchVal) {
      setSearchParams(buildSearchParams(stored), { replace: true });
      return;
    }

    searchState.writeState('searchPage', remoteKey, currentFilters);
  }, [currentFilters, queryParams, remoteKey, searchState, setSearchParams]);

  const submittedQuery = currentFilters.searchVal.trim();
  const dagPage = currentFilters.dagPage;
  const docPage = currentFilters.docPage;

  const requestParams = submittedQuery
    ? {
        params: {
          query: {
            remoteNode: remoteKey,
            q: submittedQuery,
            dagPage,
            docPage,
            perPage: SEARCH_PAGE_SIZE,
          },
        },
      }
    : {};

  const { data, isLoading } = useQuery(
    submittedQuery ? '/search' : (undefined as any), // eslint-disable-line @typescript-eslint/no-explicit-any
    requestParams,
    {
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  );

  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, []);

  const syncFilters = React.useCallback(
    (next: SearchFilters, replace = false) => {
      setSearchParams(buildSearchParams(next), { replace });
    },
    [setSearchParams]
  );

  const onSubmit = React.useCallback(
    (value: string) => {
      const nextValue = value.trim();
      syncFilters(
        {
          searchVal: nextValue,
          dagPage: 1,
          docPage: 1,
        },
        false
      );
    },
    [syncFilters]
  );

  const dagTotal = data?.dags.pagination.totalRecords ?? 0;
  const docTotal = data?.docs.pagination.totalRecords ?? 0;

  return (
    <div className="max-w-7xl">
      <div className="w-full">
        <Title>Search</Title>
        <div className="flex items-center gap-2 pt-2">
          <Input
            placeholder="Search text..."
            className="max-w-md"
            ref={ref}
            value={searchVal}
            onChange={(e) => {
              setSearchVal(e.target.value);
            }}
            type="search"
            onKeyDown={(e) => {
              if (e.key === 'Enter' && searchVal.trim()) {
                onSubmit(searchVal);
              }
            }}
          />
          <Button
            disabled={!searchVal.trim()}
            onClick={() => {
              onSubmit(searchVal);
            }}
          >
            <SearchIcon className="h-4 w-4" />
            Search
          </Button>
        </div>

        <div className="mt-4">
          {!submittedQuery && (
            <div className="text-sm text-muted-foreground italic">
              Enter a search term and press Enter or click Search
            </div>
          )}

          {submittedQuery && isLoading && !data && (
            <div className="text-sm text-muted-foreground italic">
              Searching...
            </div>
          )}

          {submittedQuery && data && dagTotal + docTotal === 0 && (
            <div className="text-sm text-muted-foreground italic">
              No results found
            </div>
          )}

          {submittedQuery && data && dagTotal + docTotal > 0 && (
            <div className="space-y-6">
              {dagTotal > 0 && (
                <div className="space-y-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <h2 className="text-lg font-semibold">
                      {dagTotal} DAG {dagTotal === 1 ? 'result' : 'results'}
                    </h2>
                    {data.dags.pagination.totalPages > 1 && (
                      <DAGPagination
                        totalPages={data.dags.pagination.totalPages}
                        page={data.dags.pagination.currentPage}
                        pageLimit={SEARCH_PAGE_SIZE}
                        pageChange={(page) => {
                          syncFilters({
                            searchVal: submittedQuery,
                            dagPage: page,
                            docPage,
                          });
                        }}
                        onPageLimitChange={() => {}}
                        showPageLimitSelector={false}
                      />
                    )}
                  </div>
                  <SearchResult
                    type="dag"
                    query={submittedQuery}
                    results={data.dags.results}
                  />
                </div>
              )}

              {docTotal > 0 && (
                <div className="space-y-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <h2 className="text-lg font-semibold">
                      {docTotal} Doc {docTotal === 1 ? 'result' : 'results'}
                    </h2>
                    {data.docs.pagination.totalPages > 1 && (
                      <DAGPagination
                        totalPages={data.docs.pagination.totalPages}
                        page={data.docs.pagination.currentPage}
                        pageLimit={SEARCH_PAGE_SIZE}
                        pageChange={(page) => {
                          syncFilters({
                            searchVal: submittedQuery,
                            dagPage,
                            docPage: page,
                          });
                        }}
                        onPageLimitChange={() => {}}
                        showPageLimitSelector={false}
                      />
                    )}
                  </div>
                  <SearchResult
                    type="doc"
                    query={submittedQuery}
                    results={data.docs.results}
                  />
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default Search;
