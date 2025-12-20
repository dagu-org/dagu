import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
  const { data } = useQuery(
    q ? '/dags/search' : (undefined as any), // eslint-disable-line @typescript-eslint/no-explicit-any
    q
      ? {
          params: {
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
              q,
            },
          },
        }
      : {},
    {
      refreshInterval: q ? 2000 : 0,
    }
  );

  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, [ref.current]);

  const onSubmit = React.useCallback((value: string) => {
    setSearchParams({
      q: value,
    });
  }, []);

  return (
    <div className="w-full">
      <div className="w-full">
        <Title>Search DAG Definitions</Title>
        <div className="flex items-center pt-2">
          <Input
            placeholder="Search text..."
            className="flex-1"
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
            variant="outline"
            className="w-24 cursor-pointer"
            onClick={async () => {
              onSubmit(searchVal);
            }}
          >
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

            if (data && data.results && data.results.length > 0) {
              return (
                <div>
                  <h2 className="text-lg font-semibold mb-2">
                    {data.results.length} results found
                  </h2>
                  <SearchResult results={data.results} />
                </div>
              );
            }

            if (
              (data && !data.results) ||
              (data && data.results && data.results.length === 0)
            ) {
              return (
                <div className="text-sm text-muted-foreground italic">
                  No results found
                </div>
              );
            }

            return null;
          })()}
        </div>
      </div>
    </div>
  );
}
export default Search;
