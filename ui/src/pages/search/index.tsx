import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Search as SearchIcon } from 'lucide-react';
import React, { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import SearchResult from '../../features/search/components/SearchResult';
import { useQuery } from '../../hooks/api';
import Title from '../../ui/Title';

function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  type SearchFilters = {
    searchVal: string;
  };

  const areFiltersEqual = (a: SearchFilters, b: SearchFilters) =>
    a.searchVal === b.searchVal;

  const defaultFilters = React.useMemo<SearchFilters>(
    () => ({
      searchVal: searchParams.get('q') || '',
    }),
    [searchParams]
  );

  const [searchVal, setSearchVal] = React.useState(defaultFilters.searchVal);

  const currentFilters = React.useMemo<SearchFilters>(
    () => ({
      searchVal,
    }),
    [searchVal]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef = React.useRef<SearchFilters | null>(null);

  React.useEffect(() => {
    const stored = searchState.readState<SearchFilters>(
      'searchPage',
      remoteKey
    );
    const hasUrl = !!searchParams.get('q');
    let next: SearchFilters;
    let shouldSyncUrl = false;

    if (hasUrl) {
      next = defaultFilters;
    } else if (stored) {
      next = {
        searchVal: stored.searchVal ?? defaultFilters.searchVal,
      };
      shouldSyncUrl = !!stored.searchVal;
    } else {
      next = defaultFilters;
      shouldSyncUrl = !!defaultFilters.searchVal;
    }

    const current = currentFiltersRef.current;
    if (current && areFiltersEqual(current, next)) {
      if (!stored || hasUrl) {
        searchState.writeState('searchPage', remoteKey, next);
      }
      lastPersistedFiltersRef.current = next;
      return;
    }

    setSearchVal(next.searchVal);
    lastPersistedFiltersRef.current = next;
    searchState.writeState('searchPage', remoteKey, next);

    if (!hasUrl && shouldSyncUrl && next.searchVal) {
      setSearchParams({ q: next.searchVal }, { replace: true });
    }
  }, [defaultFilters, remoteKey, searchParams, searchState, setSearchParams]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areFiltersEqual(persisted, currentFilters)) {
      return;
    }
    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('searchPage', remoteKey, currentFilters);
  }, [currentFilters, remoteKey, searchState]);

  const q = searchParams.get('q') || '';
  // Use a conditional key pattern - this is a standard SWR pattern for conditional fetching
  // When q is empty, we pass undefined for the first parameter, which tells SWR not to fetch
  const searchParams_ = q
    ? {
        params: {
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
            q,
          },
        },
      }
    : {};
  const searchOpts = { refreshInterval: q ? 2000 : 0 };

  const { data: dagData } = useQuery(
    q ? '/dags/search' : (undefined as any), // eslint-disable-line @typescript-eslint/no-explicit-any
    searchParams_,
    searchOpts
  );

  const { data: docData } = useQuery(
    q ? '/docs/search' : (undefined as any), // eslint-disable-line @typescript-eslint/no-explicit-any
    searchParams_,
    searchOpts
  );

  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, []);

  const onSubmit = React.useCallback((value: string) => {
    setSearchParams({
      q: value,
    });
  }, [setSearchParams]);

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
              if (e.key === 'Enter') {
                if (searchVal) {
                  onSubmit(searchVal);
                }
              }
            }}
          />
          <Button
            disabled={!searchVal}
            onClick={async () => {
              onSubmit(searchVal);
            }}
          >
            <SearchIcon className="h-4 w-4" />
            Search
          </Button>
        </div>

        <div className="mt-2">
          {(() => {
            if (!q) {
              return (
                <div className="text-sm text-muted-foreground italic">
                  Enter a search term and press Enter or click Search
                </div>
              );
            }

            const dagResults = dagData?.results ?? [];
            const docResults = docData?.results ?? [];
            const totalCount = dagResults.length + docResults.length;

            if (totalCount === 0 && dagData && docData) {
              return (
                <div className="text-sm text-muted-foreground italic">
                  No results found
                </div>
              );
            }

            return (
              <div className="space-y-4">
                {dagResults.length > 0 && (
                  <div>
                    <h2 className="text-lg font-semibold mb-2">
                      {dagResults.length} DAG{' '}
                      {dagResults.length === 1 ? 'result' : 'results'}
                    </h2>
                    <SearchResult type="dag" results={dagResults} />
                  </div>
                )}
                {docResults.length > 0 && (
                  <div>
                    <h2 className="text-lg font-semibold mb-2">
                      {docResults.length} Doc{' '}
                      {docResults.length === 1 ? 'result' : 'results'}
                    </h2>
                    <SearchResult type="doc" results={docResults} />
                  </div>
                )}
              </div>
            );
          })()}
        </div>
      </div>
    </div>
  );
}
export default Search;
