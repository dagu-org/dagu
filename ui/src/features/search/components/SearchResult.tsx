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

type Props =
  | { type: 'dag'; query: string; results: DagResult[] }
  | { type: 'doc'; query: string; results: DocResult[] };

const MATCH_PAGE_SIZE = 3;

function SearchResultItem({
  type,
  query,
  result,
}: {
  type: 'dag' | 'doc';
  query: string;
  result: DagResult | DocResult;
}) {
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const dagResult = result as DagResult;
  const docResult = result as DocResult;

  const title = type === 'dag' ? dagResult.name : docResult.title;
  const link =
    type === 'dag'
      ? `/dags/${encodeURI(dagResult.fileName)}/spec`
      : `/docs/${encodeURI(docResult.id)}`;
  const initialMatches = result.matches ?? [];
  const totalMatches = result.matchCount;

  const [matches, setMatches] = React.useState<SearchMatch[]>(initialMatches);
  const [nextPage, setNextPage] = React.useState(2);
  const [isLoadingMore, setIsLoadingMore] = React.useState(false);
  const [loadError, setLoadError] = React.useState<string | null>(null);

  React.useEffect(() => {
    setMatches(initialMatches);
    setNextPage(2);
    setIsLoadingMore(false);
    setLoadError(null);
  }, [initialMatches, query, result]);

  useEffect(() => {
    Prism.highlightAll();
  }, [matches]);

  const remainingMatches = Math.max(totalMatches - matches.length, 0);

  const loadMoreMatches = React.useCallback(async () => {
    if (isLoadingMore || remainingMatches === 0) {
      return;
    }

    setIsLoadingMore(true);
    setLoadError(null);

    try {
      const response =
        type === 'dag'
          ? await client.GET('/search/dags/{fileName}/matches', {
              params: {
                path: { fileName: dagResult.fileName },
                query: {
                  remoteNode,
                  q: query,
                  page: nextPage,
                  perPage: MATCH_PAGE_SIZE,
                },
              },
            })
          : await client.GET('/search/docs/matches', {
              params: {
                query: {
                  remoteNode,
                  path: docResult.id,
                  q: query,
                  page: nextPage,
                  perPage: MATCH_PAGE_SIZE,
                },
              },
            });

      if (response.error) {
        setLoadError(response.error.message || 'Failed to load more matches');
        return;
      }

      const newMatches = response.data?.matches ?? [];
      const pagination = response.data?.pagination;

      setMatches((current) => [...current, ...newMatches]);
      if (pagination && pagination.currentPage < pagination.totalPages) {
        setNextPage(pagination.currentPage + 1);
      }
    } catch {
      setLoadError('Failed to load more matches');
    } finally {
      setIsLoadingMore(false);
    }
  }, [client, isLoadingMore, nextPage, query, remainingMatches, remoteNode, result, type]);

  return (
    <li className="px-4 py-4">
      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between gap-4">
          <Link to={link}>
            <h3 className="text-lg font-semibold text-foreground">
              {title}
              <span className="ml-2 text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                {type === 'dag' ? 'DAG' : 'Doc'}
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

  return (
    <ul className="rounded-md border divide-y">
      {results.map((result) => {
        const key =
          type === 'dag'
            ? (result as DagResult).fileName
            : (result as DocResult).id;
        return (
          <SearchResultItem
            key={`${type}-${key}-${query}`}
            type={type}
            query={query}
            result={result}
          />
        );
      })}
    </ul>
  );
}

export default SearchResult;
