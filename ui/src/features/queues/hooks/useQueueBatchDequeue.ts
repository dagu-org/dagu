// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { components, ErrorCode } from '@/api/v1/schema';
import { QueueSelectionItem } from './useQueueSelection';

export type QueueBatchPhase = 'confirm' | 'running' | 'complete';

type ActiveBatch = {
  remoteNode: string;
  snapshot: QueueSelectionItem[];
};

export type QueueBatchResult = QueueSelectionItem & {
  ok: boolean;
  skipped?: boolean;
  error?: string;
  message?: string;
};

export type QueueBatchProgressState = {
  currentItem: QueueSelectionItem | null;
  failureCount: number;
  isRefreshing: boolean;
  processedCount: number;
  refreshError: string | null;
  results: QueueBatchResult[];
  skippedCount: number;
  successCount: number;
};

interface UseQueueBatchDequeueProps {
  onActionComplete?: () => Promise<void>;
  onReplaceSelection: (items: QueueSelectionItem[]) => void;
  selectedRuns: QueueSelectionItem[];
}

const createEmptyProgress = (): QueueBatchProgressState => ({
  currentItem: null,
  failureCount: 0,
  isRefreshing: false,
  processedCount: 0,
  refreshError: null,
  results: [],
  skippedCount: 0,
  successCount: 0,
});

const getErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof error.message === 'string' &&
    error.message.length > 0
  ) {
    return error.message;
  }
  return fallback;
};

const getSkippedResultMessage = (
  error: components['schemas']['Error']
): string | null => {
  if (error.code === ErrorCode.not_found) {
    return 'DAG run is already gone from the queue.';
  }
  if (
    error.code === ErrorCode.bad_request &&
    error.message.toLowerCase().includes('not queued')
  ) {
    return 'DAG run already left the queue.';
  }
  return null;
};

export function useQueueBatchDequeue({
  onActionComplete,
  onReplaceSelection,
  selectedRuns,
}: UseQueueBatchDequeueProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const [activeBatch, setActiveBatch] = React.useState<ActiveBatch | null>(
    null
  );
  const [phase, setPhase] = React.useState<QueueBatchPhase | null>(null);
  const [progress, setProgress] =
    React.useState<QueueBatchProgressState>(createEmptyProgress);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const isRunning = phase === 'running';

  const closeDialog = React.useCallback(() => {
    if (phase === 'running' || progress.isRefreshing) {
      return;
    }
    setActiveBatch(null);
    setPhase(null);
    setProgress(createEmptyProgress());
  }, [phase, progress.isRefreshing]);

  const openBatchDialog = React.useCallback(() => {
    if (selectedRuns.length === 0) {
      return;
    }
    setActiveBatch({
      remoteNode,
      snapshot: [...selectedRuns],
    });
    setPhase('confirm');
    setProgress(createEmptyProgress());
  }, [remoteNode, selectedRuns]);

  const submitBatchItem = React.useCallback(
    async (
      dagRun: QueueSelectionItem,
      batchRemoteNode: string
    ): Promise<QueueBatchResult> => {
      const fallback = 'Failed to dequeue DAG run';

      try {
        const { error } = await client.GET('/dag-runs/{name}/{dagRunId}/dequeue', {
          params: {
            path: {
              name: dagRun.name,
              dagRunId: dagRun.dagRunId,
            },
            query: {
              remoteNode: batchRemoteNode,
            },
          },
        });

        if (error) {
          const skippedMessage = getSkippedResultMessage(error);
          if (skippedMessage) {
            return {
              ...dagRun,
              ok: true,
              skipped: true,
              message: skippedMessage,
            };
          }

          return {
            ...dagRun,
            ok: false,
            error: error.message || fallback,
          };
        }

        return {
          ...dagRun,
          ok: true,
          message: 'Dequeue request accepted.',
        };
      } catch (error) {
        return {
          ...dagRun,
          ok: false,
          error: getErrorMessage(error, fallback),
        };
      }
    },
    [client]
  );

  const submitBatchDequeue = React.useCallback(async () => {
    if (!activeBatch) {
      return;
    }

    const { remoteNode: batchRemoteNode, snapshot } = activeBatch;
    const results: QueueBatchResult[] = [];
    let successCount = 0;
    let skippedCount = 0;
    let failureCount = 0;

    setPhase('running');
    setProgress({
      currentItem: snapshot[0] ?? null,
      failureCount: 0,
      isRefreshing: false,
      processedCount: 0,
      refreshError: null,
      results: [],
      skippedCount: 0,
      successCount: 0,
    });

    for (const [index, dagRun] of snapshot.entries()) {
      setProgress({
        currentItem: dagRun,
        failureCount,
        isRefreshing: false,
        processedCount: index,
        refreshError: null,
        results: [...results],
        skippedCount,
        successCount,
      });

      const result = await submitBatchItem(dagRun, batchRemoteNode);
      results.push(result);

      if (!result.ok) {
        failureCount++;
      } else if (result.skipped) {
        skippedCount++;
      } else {
        successCount++;
      }

      setProgress({
        currentItem: snapshot[index + 1] ?? null,
        failureCount,
        isRefreshing: false,
        processedCount: index + 1,
        refreshError: null,
        results: [...results],
        skippedCount,
        successCount,
      });
    }

    setPhase('complete');
    setProgress({
      currentItem: null,
      failureCount,
      isRefreshing: Boolean(onActionComplete),
      processedCount: snapshot.length,
      refreshError: null,
      results: [...results],
      skippedCount,
      successCount,
    });

    let refreshError: string | null = null;
    if (onActionComplete) {
      try {
        await onActionComplete();
      } catch (error) {
        refreshError = getErrorMessage(error, 'Failed to refresh queue data.');
      }
    }

    onReplaceSelection(results.filter((result) => !result.ok));

    setProgress({
      currentItem: null,
      failureCount,
      isRefreshing: false,
      processedCount: snapshot.length,
      refreshError,
      results: [...results],
      skippedCount,
      successCount,
    });
  }, [activeBatch, onActionComplete, onReplaceSelection, submitBatchItem]);

  return {
    activeBatch,
    closeDialog,
    isRunning,
    openBatchDialog,
    phase,
    progress,
    submitBatchDequeue,
  };
}
