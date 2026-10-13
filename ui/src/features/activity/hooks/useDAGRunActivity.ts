// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { useCallback, useEffect, useState } from 'react';
import type { components } from '@/api/v1/schema';
import { useQuery } from '@/hooks/api';
import { useDAGRunActivitySSE } from '@/hooks/useDAGRunActivitySSE';
import { usePageVisible } from '@/hooks/usePageVisible';
import { sseFallbackOptions, useSSECacheSync } from '@/hooks/useSSECacheSync';
import type { DAGRunActivity } from '../lib/activityModel';

/** Hours covered by the live window. */
export const LIVE_WINDOW_HOURS = 24;
/** REST polling interval while the live stream is unavailable. */
const FALLBACK_INTERVAL_MS = 5000;

/** A window that ends now and follows new runs, or a fixed past range. */
export type ActivityWindow =
  | { kind: 'live' }
  | { kind: 'fixed'; from: number; to: number };

type WorkspaceQuery = {
  workspace?: components['parameters']['Workspace'];
};

export type DAGRunActivityState = {
  activity?: DAGRunActivity;
  isLoading: boolean;
  /** Set when the node cannot serve activity, such as an older remote node. */
  unavailable: boolean;
  refresh: () => Promise<unknown>;
};

/**
 * Reads DAG-run activity for a window. A live window streams updates while
 * the tab is visible; a fixed window is read once. A node that fails the
 * read stops retrying until refresh is called, so an older remote node
 * without the endpoint is not polled forever.
 */
export function useDAGRunActivity(
  window: ActivityWindow,
  workspaceQuery: WorkspaceQuery,
  remoteNode: string
): DAGRunActivityState {
  const visible = usePageVisible();
  const live = window.kind === 'live';
  const [failedNode, setFailedNode] = useState<string | null>(null);
  const unavailable = failedNode === remoteNode;

  const sse = useDAGRunActivitySSE(
    { hours: LIVE_WINDOW_HOURS, ...workspaceQuery },
    live && visible && !unavailable,
    remoteNode
  );

  const query =
    window.kind === 'live'
      ? { hours: LIVE_WINDOW_HOURS }
      : { fromDate: window.from, toDate: window.to };
  const { data, error, isLoading, mutate } = useQuery(
    '/dag-run-activity',
    { params: { query: { ...query, ...workspaceQuery, remoteNode } } },
    {
      ...(live && !unavailable
        ? sseFallbackOptions(sse, FALLBACK_INTERVAL_MS)
        : { refreshInterval: 0, revalidateOnFocus: false }),
      keepPreviousData: true,
      shouldRetryOnError: false,
    }
  );
  useSSECacheSync(sse, mutate);

  useEffect(() => {
    if (error && !data) {
      setFailedNode(remoteNode);
    }
  }, [data, error, remoteNode]);

  const refresh = useCallback(() => {
    setFailedNode(null);
    return mutate();
  }, [mutate]);

  return {
    activity: data,
    isLoading: isLoading && !data,
    unavailable,
    refresh,
  };
}
