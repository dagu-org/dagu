import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import React, { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AppBarContext } from '../../contexts/AppBarContext';
import SearchResult from '../../features/search/components/SearchResult';
import { useQuery } from '../../hooks/api';
import LoadingIndicator from '../../ui/LoadingIndicator';
import Title from '../../ui/Title';

function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchVal, setSearchVal] = React.useState(searchParams.get('q') || '');
  const appBarContext = React.useContext(AppBarContext);

  const q = searchParams.get('q') || '';
  // Use a conditional key pattern - this is a standard SWR pattern for conditional fetching
  // When q is empty, we pass undefined for the first parameter, which tells SWR not to fetch
  const { data, isLoading } = useQuery(
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

  if (q && isLoading) {
    return <LoadingIndicator />;
  }

  return (
    <div className="w-full">
      <div className="w-full">
        <Title>Search DAG Definitions</Title>
        <div className="flex space-x-4 items-center">
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

        <div className="mt-4">
          {(() => {
            if (!q) {
              return (
                <div className="text-sm text-gray-500 italic">
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
                <div className="text-sm text-gray-500 italic">
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
