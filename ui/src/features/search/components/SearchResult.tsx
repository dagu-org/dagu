import { Button } from '@/components/ui/button';
import React, { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { components } from '../../../api/v1/schema';
import Prism from '../../../assets/js/prism';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useClient } from '../../../hooks/api';
import { DAGDefinition } from '../../dags/components/dag-editor';

type SearchMatch = components['schemas']['SearchDAGsMatchItem'];
type DagResult = components['schemas']['DAGSearchPageItem'];
type DocResult = components['schemas']['DocSearchPageItem'];

type BaseSearchResultItemProps = {
  kind: 'DAG' | 'Doc';
  title: string;
  link: string;
  query: string;
  totalMatches: number;
  initialMatches: SearchMatch[];
  loadPage: (page: number) => Promise<{
    error?: string;
    matches: SearchMatch[];
    hasMore: boolean;
  }>;
};

type Props =
  | { type: 'dag'; query: string; results: DagResult[] }
  | { type: 'doc'; query: string; results: DocResult[] };

const MATCH_PAGE_SIZE = 3;

function SearchResultItem({
  kind,
  title,
  link,
  query,
  totalMatches,
  initialMatches,
  loadPage,
}: BaseSearchResultItemProps) {
  const [matches, setMatches] = React.useState<SearchMatch[]>(initialMatches);
  const [nextPage, setNextPage] = React.useState(2);
  const [hasMoreMatches, setHasMoreMatches] = React.useState(
    totalMatches > initialMatches.length
  );
  const [isLoadingMore, setIsLoadingMore] = React.useState(false);
  const [loadError, setLoadError] = React.useState<string | null>(null);

  React.useEffect(() => {
    setMatches(initialMatches);
    setNextPage(2);
    setHasMoreMatches(totalMatches > initialMatches.length);
    setIsLoadingMore(false);
    setLoadError(null);
  }, [initialMatches, query, totalMatches]);

  useEffect(() => {
    Prism.highlightAll();
  }, [matches]);

  const remainingMatches = Math.max(totalMatches - matches.length, 0);

  const loadMoreMatches = React.useCallback(async () => {
    if (isLoadingMore || !hasMoreMatches) {
      return;
    }

    setIsLoadingMore(true);
    setLoadError(null);

    try {
      const response = await loadPage(nextPage);
      if (response.error) {
        setLoadError(response.error);
        return;
      }

      setMatches((current) => [...current, ...response.matches]);
      setHasMoreMatches(response.hasMore);
      if (response.hasMore) {
        setNextPage((current) => current + 1);
      }
    } catch {
      setLoadError('Failed to load more matches');
    } finally {
      setIsLoadingMore(false);
    }
  }, [hasMoreMatches, isLoadingMore, loadPage, nextPage]);

  return (
    <li className="px-4 py-4">
      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between gap-4">
          <Link to={link}>
            <h3 className="text-lg font-semibold text-foreground">
              {title}
              <span className="ml-2 text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                {kind}
              </span>
            </h3>
          </Link>
          <span className="text-xs text-muted-foreground">
            {totalMatches} {totalMatches === 1 ? 'match' : 'matches'}
          </span>
        </div>

        {matches.map((match, index) => (
          <DAGDefinition
            key={`${link}-${match.lineNumber}-${index}`}
            value={match.line}
            lineNumbers
            startLine={match.startLine}
            highlightLine={match.lineNumber - match.startLine}
            noHighlight
          />
        ))}

        {loadError && (
          <div className="text-sm text-destructive">{loadError}</div>
        )}

        {remainingMatches > 0 && (
          <div>
            <Button
              variant="outline"
              size="sm"
              disabled={isLoadingMore}
              onClick={() => {
                void loadMoreMatches();
              }}
            >
              {isLoadingMore
                ? 'Loading...'
                : `Show ${Math.min(remainingMatches, MATCH_PAGE_SIZE)} more matches`}
            </Button>
          </div>
        )}
      </div>
    </li>
  );
}

function SearchResult(props: Props) {
  const { type, query, results } = props;
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  return (
    <ul className="rounded-md border divide-y">
      {type === 'dag'
        ? results.map((result) => (
            <SearchResultItem
              key={`dag-${result.fileName}-${query}`}
              kind="DAG"
              title={result.name}
              link={`/dags/${encodeURI(result.fileName)}/spec`}
              query={query}
              totalMatches={result.matchCount}
              initialMatches={result.matches ?? []}
              loadPage={async (page) => {
                const response = await client.GET(
                  '/search/dags/{fileName}/matches',
                  {
                    params: {
                      path: { fileName: result.fileName },
                      query: {
                        remoteNode,
                        q: query,
                        page,
                        perPage: MATCH_PAGE_SIZE,
                      },
                    },
                  }
                );

                return {
                  error:
                    response.error?.message || undefined,
                  matches: response.data?.matches ?? [],
                  hasMore:
                    !!response.data?.pagination &&
                    response.data.pagination.currentPage <
                      response.data.pagination.totalPages,
                };
              }}
            />
          ))
        : results.map((result) => (
            <SearchResultItem
              key={`doc-${result.id}-${query}`}
              kind="Doc"
              title={result.title}
              link={`/docs/${encodeURI(result.id)}`}
              query={query}
              totalMatches={result.matchCount}
              initialMatches={result.matches ?? []}
              loadPage={async (page) => {
                const response = await client.GET('/search/docs/matches', {
                  params: {
                    query: {
                      remoteNode,
                      path: result.id,
                      q: query,
                      page,
                      perPage: MATCH_PAGE_SIZE,
                    },
                  },
                });

                return {
                  error:
                    response.error?.message || undefined,
                  matches: response.data?.matches ?? [],
                  hasMore:
                    !!response.data?.pagination &&
                    response.data.pagination.currentPage <
                      response.data.pagination.totalPages,
                };
              }}
            />
          ))}
    </ul>
  );
}

export default SearchResult;
