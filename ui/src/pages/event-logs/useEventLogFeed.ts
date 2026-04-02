import { useClient, useQuery } from '@/hooks/api';
import * as React from 'react';
import type { EventLogEntry, EventLogQueryParams, EventLogsResponse } from './types';
import {
  appendUniqueEntries,
  getClientErrorMessage,
  mergeUniqueEntries,
} from './utils';

export function useEventLogFeed(
  client: ReturnType<typeof useClient>,
  query: EventLogQueryParams,
  isReady: boolean
) {
  const [autoRefresh, setAutoRefresh] = React.useState(true);
  const [lastUpdatedAt, setLastUpdatedAt] = React.useState<Date | null>(null);
  const [olderEntries, setOlderEntries] = React.useState<EventLogEntry[]>([]);
  const [continuationCursorOverride, setContinuationCursorOverride] =
    React.useState<string | null | undefined>(undefined);
  const [isLoadingMore, setIsLoadingMore] = React.useState(false);
  const [loadMoreError, setLoadMoreError] = React.useState<string | null>(null);
  const activeFeedKeyRef = React.useRef('');
  const feedKey = React.useMemo(() => JSON.stringify(query), [query]);

  const { data, error, isLoading, mutate } = useQuery(
    '/event-logs',
    isReady
      ? {
          params: {
            query,
          },
        }
      : null,
    {
      refreshInterval: autoRefresh ? 5000 : 0,
      revalidateOnFocus: true,
      revalidateOnReconnect: true,
    }
  );

  React.useEffect(() => {
    activeFeedKeyRef.current = feedKey;
    setOlderEntries([]);
    setContinuationCursorOverride(undefined);
    setLoadMoreError(null);
    setIsLoadingMore(false);
    setAutoRefresh(true);
  }, [feedKey]);

  const headEntries = data?.entries ?? [];
  const entries = React.useMemo(
    () => mergeUniqueEntries(headEntries, olderEntries),
    [headEntries, olderEntries]
  );
  const headFirstEntryID = headEntries[0]?.id ?? '';
  const headLastEntryID =
    headEntries.length > 0 ? (headEntries[headEntries.length - 1]?.id ?? '') : '';
  const headNextCursor = data?.nextCursor ?? '';
  const currentNextCursor =
    continuationCursorOverride === undefined
      ? (data?.nextCursor ?? null)
      : continuationCursorOverride;
  const hasLoadedMore = continuationCursorOverride !== undefined;
  const hasMoreEntries = currentNextCursor !== null;
  const hasHeadResponse = data !== undefined;
  const isAutoRefreshAvailable = !hasLoadedMore;

  React.useEffect(() => {
    if (hasHeadResponse) {
      setLastUpdatedAt(new Date());
    }
  }, [hasHeadResponse, headFirstEntryID, headLastEntryID, headNextCursor]);

  const handleRefresh = React.useCallback(async () => {
    setOlderEntries([]);
    setContinuationCursorOverride(undefined);
    setLoadMoreError(null);
    setIsLoadingMore(false);
    setAutoRefresh(true);
    await mutate();
  }, [mutate]);

  const handleLoadMore = React.useCallback(async () => {
    if (isLoadingMore || !currentNextCursor) {
      return;
    }

    const requestFeedKey = activeFeedKeyRef.current;
    setIsLoadingMore(true);
    setLoadMoreError(null);
    setAutoRefresh(false);

    const response = await client.GET('/event-logs', {
      params: {
        query: {
          ...query,
          cursor: currentNextCursor,
        },
      },
    });

    if (activeFeedKeyRef.current !== requestFeedKey) {
      return;
    }

    setIsLoadingMore(false);

    if (response.error) {
      setLoadMoreError(
        getClientErrorMessage(response.error, 'Failed to load older events')
      );
      return;
    }

    const pageData = (response.data ?? { entries: [] }) as EventLogsResponse;
    setOlderEntries((prev) =>
      appendUniqueEntries(prev, pageData.entries ?? [])
    );
    setContinuationCursorOverride(pageData.nextCursor ?? null);
  }, [client, currentNextCursor, isLoadingMore, query]);

  return {
    data,
    error,
    isLoading,
    autoRefresh,
    setAutoRefresh,
    lastUpdatedAt,
    entries,
    hasLoadedMore,
    hasMoreEntries,
    isAutoRefreshAvailable,
    isLoadingMore,
    loadMoreError,
    handleRefresh,
    handleLoadMore,
  };
}
