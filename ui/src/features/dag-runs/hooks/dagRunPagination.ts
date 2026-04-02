import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { components, paths } from '@/api/v1/schema';
import { useClient } from '@/hooks/api';
import { useLiveConnection, useLiveInvalidation } from '@/hooks/useAppLive';
import { isAbortLikeError } from '@/lib/requestTimeout';

export type DAGRunSummary = components['schemas']['DAGRunSummary'];
export type DAGRunsPageResponse = components['schemas']['DAGRunsPageResponse'];
export type DAGRunListQuery = paths['/dag-runs']['get']['parameters']['query'];

const MAX_DAG_RUN_PAGE_LIMIT = 500;

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
  const queryKey = useMemo(() => JSON.stringify(query), [query]);

  dataRef.current = data;

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
      const next = await fetchAllDAGRuns(client, query, controller.signal);
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
  }, [client, enabled, query]);

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
