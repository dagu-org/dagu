import React, { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import Title from '../../ui/Title';
import SearchResult from '../../features/search/components/SearchResult';
import LoadingIndicator from '../../ui/LoadingIndicator';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useQuery } from '../../hooks/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchVal, setSearchVal] = React.useState(searchParams.get('q') || '');
  const appBarContext = React.useContext(AppBarContext);

  const { data, isLoading } = useQuery(
    '/dags/search',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          q: searchParams.get('q') || '',
        },
      },
    },
    { refreshInterval: 2000 }
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

  if (isLoading) {
    return <LoadingIndicator />;
  }

  return (
    <div className="w-full">
      <div className="w-full">
        <Title>Search</Title>
        <div className="flex space-x-4 items-center">
          <Input
            placeholder="Search Text"
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
            className="w-24"
            onClick={async () => {
              onSubmit(searchVal);
            }}
          >
            Search
          </Button>
        </div>

        <div className="mt-4">
          {(() => {
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
              return <div>No results found</div>;
            }

            return null;
          })()}
        </div>
      </div>
    </div>
  );
}
export default Search;
