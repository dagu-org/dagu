import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { components } from '@/api/v1/schema';
import SearchResult from '@/features/search/components/SearchResult';
import { useInfinite } from '@/hooks/api';
import { Search as SearchIcon } from 'lucide-react';
import React, { useEffect, useMemo, useRef } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import Title from '../../ui/Title';

const SEARCH_PAGE_SIZE = 20;

type SearchScope = 'dags' | 'docs';

type SearchFilters = {
  searchVal: string;
  scope: SearchScope;
};

function parseScope(value: string | null): SearchScope {
  return value === 'docs' ? 'docs' : 'dags';
}

function buildSearchParams(filters: SearchFilters): URLSearchParams {
  const params = new URLSearchParams();
  const query = filters.searchVal.trim();

  if (query) {
    params.set('q', query);
    params.set('scope', filters.scope);
    return params;
  }

  if (filters.scope !== 'dags') {
    params.set('scope', filters.scope);
  }

  return params;
}

function Search() {
  const [, setSearchParams] = useSearchParams();
  const location = useLocation();
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';
  const inputRef = useRef<HTMLInputElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );

  const currentFilters = useMemo<SearchFilters>(
    () => ({
      searchVal: queryParams.get('q') || '',
      scope: parseScope(queryParams.get('scope')),
    }),
    [queryParams]
  );

  const [searchVal, setSearchVal] = React.useState(currentFilters.searchVal);

  useEffect(() => {
    setSearchVal(currentFilters.searchVal);
  }, [currentFilters.searchVal]);

  useEffect(() => {
    const hasUrlState = queryParams.has('q') || queryParams.has('scope');
    const stored = searchState.readState<SearchFilters>('searchPage', remoteKey);

    if (!hasUrlState && stored) {
      setSearchParams(buildSearchParams(stored), { replace: true });
      return;
    }

    searchState.writeState('searchPage', remoteKey, currentFilters);
  }, [currentFilters, queryParams, remoteKey, searchState, setSearchParams]);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const submittedQuery = currentFilters.searchVal.trim();
  const endpoint =
    currentFilters.scope === 'docs' ? '/search/docs' : '/search/dags';

  const syncFilters = React.useCallback(
    (next: SearchFilters, replace = false) => {
      setSearchParams(buildSearchParams(next), { replace });
    },
    [setSearchParams]
  );

  const onSubmit = React.useCallback(
    (value: string) => {
      syncFilters(
        {
          searchVal: value.trim(),
          scope: currentFilters.scope,
        },
        false
      );
    },
    [currentFilters.scope, syncFilters]
  );

  const { data, error, isLoading, isValidating, size, setSize } = useInfinite(
    endpoint,
    (pageIndex, previousPage) => {
      if (!submittedQuery) {
        return null;
      }
      if (previousPage && !previousPage.hasMore) {
        return null;
      }

      return {
        params: {
          query: {
            remoteNode: remoteKey,
            q: submittedQuery,
            limit: SEARCH_PAGE_SIZE,
            cursor: pageIndex === 0 ? undefined : previousPage?.nextCursor,
          },
        },
      };
    },
    {
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      revalidateFirstPage: false,
    }
  );

  type SearchPage =
    | components['schemas']['DAGSearchFeedResponse']
    | components['schemas']['DocSearchFeedResponse'];

  const pages = (data ?? []) as SearchPage[];
  const dagResults =
    currentFilters.scope === 'dags'
      ? pages.flatMap(
          (page) =>
            (page as components['schemas']['DAGSearchFeedResponse']).results ??
            []
        )
      : [];
  const docResults =
    currentFilters.scope === 'docs'
      ? pages.flatMap(
          (page) =>
            (page as components['schemas']['DocSearchFeedResponse']).results ??
            []
        )
      : [];
  const hasResults =
    currentFilters.scope === 'docs' ? docResults.length > 0 : dagResults.length > 0;
  const lastPage = pages[pages.length - 1];
  const hasMore = lastPage?.hasMore ?? false;
  const isLoadingMore = isValidating && pages.length > 0;

  const loadMoreResults = React.useCallback(() => {
    if (!submittedQuery || !hasMore || isLoadingMore) {
      return;
    }
    void setSize((current) => current + 1);
  }, [hasMore, isLoadingMore, setSize, submittedQuery]);

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !submittedQuery || !hasMore) {
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          loadMoreResults();
        }
      },
      { threshold: 0.1 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasMore, loadMoreResults, submittedQuery]);
  const scopeTitle = currentFilters.scope === 'docs' ? 'Documents' : 'DAGs';

  return (
    <div className="max-w-5xl">
      <div className="w-full">
        <Title>Search</Title>

        <div className="flex flex-col gap-3 pt-2">
          <div className="flex flex-wrap items-center gap-2">
            <Input
              placeholder="Search text..."
              className="max-w-md"
              ref={inputRef}
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

          <ToggleGroup aria-label="Search scope">
            <ToggleButton
              value="dags"
              groupValue={currentFilters.scope}
              onClick={() => {
                syncFilters({
                  searchVal: currentFilters.searchVal,
                  scope: 'dags',
                });
              }}
            >
              DAGs
            </ToggleButton>
            <ToggleButton
              value="docs"
              groupValue={currentFilters.scope}
              onClick={() => {
                syncFilters({
                  searchVal: currentFilters.searchVal,
                  scope: 'docs',
                });
              }}
            >
              Docs
            </ToggleButton>
          </ToggleGroup>
        </div>

        <div className="mt-4 space-y-4">
          {!submittedQuery && (
            <div className="text-sm text-muted-foreground italic">
              Enter a search term and press Enter or click Search
            </div>
          )}

          {submittedQuery && isLoading && !pages.length && (
            <div className="text-sm text-muted-foreground italic">
              Searching {scopeTitle.toLowerCase()}...
            </div>
          )}

          {submittedQuery && error && !pages.length && (
            <div className="text-sm text-destructive">
              Search failed. Try again.
            </div>
          )}

          {submittedQuery && !isLoading && !hasResults && !error && (
            <div className="text-sm text-muted-foreground italic">
              No {scopeTitle.toLowerCase()} found
            </div>
          )}

          {submittedQuery && hasResults && (
            <div className="space-y-4">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-lg font-semibold">{scopeTitle}</h2>
                <span className="text-xs text-muted-foreground">
                  Infinite results
                </span>
              </div>

              {currentFilters.scope === 'docs' ? (
                <SearchResult
                  type="doc"
                  query={submittedQuery}
                  results={docResults}
                />
              ) : (
                <SearchResult
                  type="dag"
                  query={submittedQuery}
                  results={dagResults}
                />
              )}

              {hasMore && (
                <div className="flex flex-col items-center gap-3">
                  <Button
                    variant="outline"
                    onClick={() => {
                      loadMoreResults();
                    }}
                    disabled={isLoadingMore}
                  >
                    {isLoadingMore ? 'Loading...' : 'Load more'}
                  </Button>
                  <div ref={sentinelRef} className="h-4 w-full" />
                </div>
              )}

              {!hasMore && pages.length > 0 && size > 0 && (
                <div className="text-center text-xs text-muted-foreground">
                  End of results
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
