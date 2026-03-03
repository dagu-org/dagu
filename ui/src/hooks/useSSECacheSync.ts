import { useEffect, useRef } from 'react';
import type { KeyedMutator } from 'swr';
import type { SSEState } from './useSSE';

/**
 * Returns SWR config that polls when SSE is inactive and stops polling when
 * SSE is connected. Unlike the previous pattern, does NOT use `isPaused`,
 * so `mutate()` always works (critical for post-save cache updates).
 */
export function sseFallbackOptions(
  sseResult: SSEState<unknown>,
  fallbackInterval: number = 2000
) {
  const sseActive = sseResult.isConnected && !sseResult.shouldUseFallback;
  return {
    revalidateOnMount: true,
    revalidateIfStale: !sseActive,
    revalidateOnFocus: !sseActive,
    refreshInterval: sseActive ? 0 : fallbackInterval,
  };
}

/**
 * Pushes SSE data into the SWR cache so that SWR is the single source of truth.
 * When SSE disconnects, the SWR cache already has the latest data — no stale
 * fallback. Uses reference equality for change detection; since JSON.parse
 * creates new objects each SSE event, this triggers mutate on every data event.
 * This is acceptable because mutate(data, { revalidate: false }) is cheap
 * (no network request).
 *
 * An optional `transform` function maps SSE data to the SWR cache shape when
 * the SSE type differs from the SWR endpoint type (e.g., DAGSpec).
 */
export function useSSECacheSync<S, T = S>(
  sseResult: SSEState<S>,
  mutate: KeyedMutator<T>,
  transform?: (data: S) => T
) {
  const prevDataRef = useRef<S | null>(null);
  useEffect(() => {
    if (
      sseResult.isConnected &&
      sseResult.data != null &&
      sseResult.data !== prevDataRef.current
    ) {
      prevDataRef.current = sseResult.data;
      const cacheData = transform
        ? transform(sseResult.data)
        : (sseResult.data as unknown as T);
      mutate(cacheData, { revalidate: false });
    }
  }, [sseResult.data, sseResult.isConnected, mutate, transform]);
}
