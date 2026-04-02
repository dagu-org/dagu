import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { components, paths } from '@/api/v1/schema';
import { useClient, useQuery } from '@/hooks/api';
import {
  liveFallbackOptions,
  useLiveConnection,
  useLiveDAGRuns,
  useLiveInvalidation,
} from '@/hooks/useAppLive';
import { isAbortLikeError } from '@/lib/requestTimeout';

export type DAGRunSummary = components['schemas']['DAGRunSummary'];
export type DAGRunsPageResponse = components['schemas']['DAGRunsPageResponse'];
export type DAGRunListQuery = paths['/dag-runs']['get']['parameters']['query'];

const MAX_DAG_RUN_PAGE_LIMIT = 500;

function normalizeDAGRunListQuery(
  query: DAGRunListQuery | undefined
): Record<string, unknown> {
  const normalizedEntries = Object.entries(query ?? {})
    .filter(([, value]) => value !== undefined)
    .sort(([left], [right]) => left.localeCompare(right));
  return Object.fromEntries(normalizedEntries);
}

function getDAGRunListQueryKey(query: DAGRunListQuery | undefined): string {
  return JSON.stringify(normalizeDAGRunListQuery(query));
}

export function mergeUniqueDAGRuns(
  head: DAGRunSummary[],
  older: DAGRunSummary[]
): DAGRunSummary[] {
  const merged: DAGRunSummary[] = [];
  const seen = new Set<string>();

  for (const dagRun of [...head, ...older]) {
    const key = `${dagRun.name}\u0000${dagRun.dagRunId}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    merged.push(dagRun);
  }

  return merged;
}

async function fetchDAGRunsPage(
  client: ReturnType<typeof useClient>,
  query: DAGRunListQuery,
  signal: AbortSignal
): Promise<DAGRunsPageResponse> {
  const response = await client.GET('/dag-runs', {
    params: { query },
    signal,
  });

  if (response.error) {
    const message =
      response.error &&
      typeof response.error === 'object' &&
      'message' in response.error
        ? String(response.error.message)
        : 'Failed to load DAG runs';
    throw new Error(message);
  }

  return (response.data ?? { dagRuns: [] }) as DAGRunsPageResponse;
}

export async function fetchAllDAGRuns(
  client: ReturnType<typeof useClient>,
  query: DAGRunListQuery,
  signal: AbortSignal
): Promise<DAGRunSummary[]> {
  const allRuns: DAGRunSummary[] = [];
  let cursor: string | undefined;

  for (;;) {
    if (signal.aborted) {
      throw new DOMException('Aborted', 'AbortError');
    }

    const page = await fetchDAGRunsPage(
      client,
      {
        ...query,
        limit: MAX_DAG_RUN_PAGE_LIMIT,
        cursor,
      },
      signal
    );

    allRuns.push(...(page.dagRuns ?? []));
    if (!page.nextCursor) {
      return allRuns;
    }
    cursor = page.nextCursor;
  }
}

type UseExactDAGRunsOptions = {
  query: DAGRunListQuery;
  enabled?: boolean;
  liveEnabled?: boolean;
  fallbackIntervalMs?: number;
};

type UsePaginatedDAGRunsOptions = {
  query: DAGRunListQuery;
  enabled?: boolean;
};

type UsePaginatedDAGRunsResult = {
  dagRuns: DAGRunSummary[];
  headPage: DAGRunsPageResponse | undefined;
  isInitialLoading: boolean;
  isLoadingMore: boolean;
  loadMoreError: string | null;
  hasMore: boolean;
  refresh: () => Promise<void>;
  loadMore: () => Promise<void>;
};

type UseExactDAGRunsResult = {
  data: DAGRunSummary[];
  error: Error | null;
  isLoading: boolean;
  isValidating: boolean;
  refresh: () => Promise<void>;
};

export function useExactDAGRuns({
  query,
  enabled = true,
  liveEnabled = true,
  fallbackIntervalMs = 5000,
}: UseExactDAGRunsOptions): UseExactDAGRunsResult {
  const client = useClient();
  const liveState = useLiveConnection(enabled && liveEnabled);
  const [data, setData] = useState<DAGRunSummary[]>([]);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  const dataRef = useRef<DAGRunSummary[]>([]);
  const controllerRef = useRef<AbortController | null>(null);
  const requestIDRef = useRef(0);
  const queryRef = useRef(query);
  const queryKey = useMemo(() => getDAGRunListQueryKey(query), [query]);

  dataRef.current = data;
  queryRef.current = query;

  const refresh = useCallback(async (): Promise<void> => {
    if (!enabled) {
      return;
    }

    const requestID = requestIDRef.current + 1;
    requestIDRef.current = requestID;

    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    setError(null);

    if (dataRef.current.length === 0) {
      setIsLoading(true);
    } else {
      setIsValidating(true);
    }

    try {
      const next = await fetchAllDAGRuns(
        client,
        queryRef.current,
        controller.signal
      );
      if (controller.signal.aborted || requestID !== requestIDRef.current) {
        return;
      }
      dataRef.current = next;
      setData(next);
      setError(null);
    } catch (fetchError) {
      if (
        (controller.signal.aborted && isAbortLikeError(fetchError)) ||
        requestID !== requestIDRef.current
      ) {
        return;
      }
      setError(
        fetchError instanceof Error
          ? fetchError
          : new Error('Failed to load DAG runs')
      );
    } finally {
      if (requestID === requestIDRef.current) {
        if (controllerRef.current === controller) {
          controllerRef.current = null;
        }
        setIsLoading(false);
        setIsValidating(false);
      }
    }
  }, [client, enabled]);

  useEffect(() => {
    if (!enabled) {
      controllerRef.current?.abort();
      controllerRef.current = null;
      setData([]);
      setError(null);
      setIsLoading(false);
      setIsValidating(false);
      return;
    }

    // Trigger reloads only when the semantic query changes, not when callers
    // allocate a fresh but equivalent query object during rerenders.
    void refresh();

    return () => {
      controllerRef.current?.abort();
      controllerRef.current = null;
    };
  }, [enabled, queryKey, refresh]);

  useEffect(() => {
    if (
      !enabled ||
      fallbackIntervalMs <= 0 ||
      (liveEnabled && liveState.isConnected && !liveState.shouldUseFallback)
    ) {
      return;
    }

    const timer = window.setInterval(() => {
      void refresh();
    }, fallbackIntervalMs);

    return () => {
      window.clearInterval(timer);
    };
  }, [
    enabled,
    fallbackIntervalMs,
    liveEnabled,
    liveState.isConnected,
    liveState.shouldUseFallback,
    refresh,
  ]);

  const liveMutate = useCallback(async () => {
    await refresh();
    return {
      dagRuns: dataRef.current,
    } as DAGRunsPageResponse;
  }, [refresh]);

  useLiveInvalidation({
    enabled: enabled && liveEnabled,
    mutate: liveMutate,
    matcher: (event) =>
      event.type === 'reset' || event.type === 'dagrun.changed',
  });

  return {
    data,
    error,
    isLoading,
    isValidating,
    refresh,
  };
}

export function usePaginatedDAGRuns({
  query,
  enabled = true,
}: UsePaginatedDAGRunsOptions): UsePaginatedDAGRunsResult {
  const client = useClient();
  const liveState = useLiveConnection(enabled);
  const [olderRuns, setOlderRuns] = useState<DAGRunSummary[]>([]);
  const [continuationCursorOverride, setContinuationCursorOverride] = useState<
    string | null | undefined
  >(undefined);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState<string | null>(null);

  const stableQueryKey = useMemo(() => getDAGRunListQueryKey(query), [query]);
  const {
    data: headPage,
    mutate,
    isLoading,
  } = useQuery(
    '/dag-runs',
    enabled
      ? {
          params: {
            query,
          },
        }
      : null,
    liveFallbackOptions(liveState)
  );
  useLiveDAGRuns(mutate, enabled);

  useEffect(() => {
    setOlderRuns([]);
    setContinuationCursorOverride(undefined);
    setLoadMoreError(null);
    setIsLoadingMore(false);
  }, [stableQueryKey]);

  const dagRuns = useMemo(
    () => mergeUniqueDAGRuns(headPage?.dagRuns ?? [], olderRuns),
    [headPage?.dagRuns, olderRuns]
  );
  const nextCursor =
    continuationCursorOverride === undefined
      ? (headPage?.nextCursor ?? null)
      : continuationCursorOverride;

  const refresh = useCallback(async (): Promise<void> => {
    setOlderRuns([]);
    setContinuationCursorOverride(undefined);
    setLoadMoreError(null);
    setIsLoadingMore(false);
    await mutate();
  }, [mutate]);

  const loadMore = useCallback(async (): Promise<void> => {
    if (isLoadingMore || !nextCursor) {
      return;
    }

    setIsLoadingMore(true);
    setLoadMoreError(null);

    const response = await client.GET('/dag-runs', {
      params: {
        query: {
          ...query,
          cursor: nextCursor,
        },
      },
    });

    setIsLoadingMore(false);

    if (response.error) {
      const message =
        response.error &&
        typeof response.error === 'object' &&
        'message' in response.error
          ? String(response.error.message)
          : 'Failed to load more DAG runs';
      setLoadMoreError(message);
      return;
    }

    const pageData = (response.data ?? { dagRuns: [] }) as DAGRunsPageResponse;
    setOlderRuns((previous) =>
      mergeUniqueDAGRuns(previous, pageData.dagRuns ?? [])
    );
    setContinuationCursorOverride(pageData.nextCursor ?? null);
  }, [client, isLoadingMore, nextCursor, query]);

  return {
    dagRuns,
    headPage,
    isInitialLoading: isLoading,
    isLoadingMore,
    loadMoreError,
    hasMore: nextCursor !== null,
    refresh,
    loadMore,
  };
}
