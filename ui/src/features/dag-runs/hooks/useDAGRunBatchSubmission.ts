// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { DAGRunSelectionItem } from './useBulkDAGRunSelection';

export type BatchActionType = 'retry' | 'reschedule' | 'delete';
export type BatchActionPhase = 'confirm' | 'running' | 'complete';

type ActiveBatch = {
  action: BatchActionType;
  snapshot: DAGRunSelectionItem[];
};

type BatchActionOptions = {
  useCurrentDagFile?: boolean;
};

export type BatchActionResult = DAGRunSelectionItem & {
  ok: boolean;
  error?: string;
  newDagRunId?: string;
  queued?: boolean;
};

export type BatchProgressState = {
  currentItem: DAGRunSelectionItem | null;
  failureCount: number;
  isRefreshing: boolean;
  processedCount: number;
  refreshError: string | null;
  results: BatchActionResult[];
  successCount: number;
};

interface UseDAGRunBatchSubmissionProps {
  onActionComplete?: () => Promise<void>;
  onReplaceSelection: (items: DAGRunSelectionItem[]) => void;
  selectedRuns: DAGRunSelectionItem[];
}

const createEmptyProgress = (): BatchProgressState => ({
  currentItem: null,
  failureCount: 0,
  isRefreshing: false,
  processedCount: 0,
  refreshError: null,
  results: [],
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

const assertUnreachable = (value: never): never => {
  throw new Error(`Unsupported DAG-run batch action: ${String(value)}`);
};

export function useDAGRunBatchSubmission({
  onActionComplete,
  onReplaceSelection,
  selectedRuns,
}: UseDAGRunBatchSubmissionProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const [activeBatch, setActiveBatch] = React.useState<ActiveBatch | null>(
    null
  );
  const [phase, setPhase] = React.useState<BatchActionPhase | null>(null);
  const [progress, setProgress] =
    React.useState<BatchProgressState>(createEmptyProgress);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const isRunning = phase === 'running';

  const closeDialog = React.useCallback(() => {
    if (phase === 'running') {
      return;
    }
    setActiveBatch(null);
    setPhase(null);
    setProgress(createEmptyProgress());
  }, [phase]);

  const openBatchDialog = React.useCallback(
    (action: BatchActionType) => {
      if (selectedRuns.length === 0) {
        return;
      }
      setActiveBatch({
        action,
        snapshot: [...selectedRuns],
      });
      setPhase('confirm');
      setProgress(createEmptyProgress());
    },
    [selectedRuns]
  );

  const submitBatchItem = React.useCallback(
    async (
      action: BatchActionType,
      dagRun: DAGRunSelectionItem,
      options?: BatchActionOptions
    ): Promise<BatchActionResult> => {
      const fallback = `Failed to ${action} DAG run`;

      try {
        switch (action) {
          case 'retry': {
            const { error } = await client.POST(
              '/dag-runs/{name}/{dagRunId}/retry',
              {
                params: {
                  path: {
                    name: dagRun.name,
                    dagRunId: dagRun.dagRunId,
                  },
                  query: {
                    remoteNode,
                  },
                },
                body: {
                  dagRunId: dagRun.dagRunId,
                },
              }
            );

            if (error) {
              return {
                ...dagRun,
                ok: false,
                error: error.message || fallback,
              };
            }

            return {
              ...dagRun,
              ok: true,
            };
          }

          case 'delete': {
            const { error } = await client.DELETE(
              '/dag-runs/{name}/{dagRunId}',
              {
                params: {
                  path: {
                    name: dagRun.name,
                    dagRunId: dagRun.dagRunId,
                  },
                  query: {
                    remoteNode,
                  },
                },
              }
            );

            if (error) {
              return {
                ...dagRun,
                ok: false,
                error: error.message || fallback,
              };
            }

            return {
              ...dagRun,
              ok: true,
            };
          }

          case 'reschedule': {
            const { data, error } = await client.POST(
              '/dag-runs/{name}/{dagRunId}/reschedule',
              {
                params: {
                  path: {
                    name: dagRun.name,
                    dagRunId: dagRun.dagRunId,
                  },
                  query: {
                    remoteNode,
                  },
                },
                body: {
                  dagRunId: undefined,
                  useCurrentDagFile: options?.useCurrentDagFile,
                },
              }
            );

            if (error) {
              return {
                ...dagRun,
                ok: false,
                error: error.message || fallback,
              };
            }

            if (!data?.dagRunId) {
              return {
                ...dagRun,
                ok: false,
                error: 'Reschedule request did not return a new DAG run ID.',
              };
            }

            return {
              ...dagRun,
              ok: true,
              newDagRunId: data.dagRunId,
              queued: data?.queued,
            };
          }

          default:
            return assertUnreachable(action);
        }
      } catch (error) {
        return {
          ...dagRun,
          ok: false,
          error: getErrorMessage(error, fallback),
        };
      }
    },
    [client, remoteNode]
  );

  const submitBatchAction = React.useCallback(
    async (options?: BatchActionOptions) => {
      if (!activeBatch) {
        return;
      }

      const { action, snapshot } = activeBatch;
      const results: BatchActionResult[] = [];
      let successCount = 0;
      let failureCount = 0;

      setPhase('running');
      setProgress({
        currentItem: snapshot[0] ?? null,
        failureCount: 0,
        isRefreshing: false,
        processedCount: 0,
        refreshError: null,
        results: [],
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
          successCount,
        });

        const result = await submitBatchItem(action, dagRun, options);
        results.push(result);

        if (result.ok) {
          successCount++;
        } else {
          failureCount++;
        }

        setProgress({
          currentItem: snapshot[index + 1] ?? null,
          failureCount,
          isRefreshing: false,
          processedCount: index + 1,
          refreshError: null,
          results: [...results],
          successCount,
        });
      }

      setPhase('complete');
      setProgress({
        currentItem: null,
        failureCount,
        isRefreshing: true,
        processedCount: snapshot.length,
        refreshError: null,
        results,
        successCount,
      });

      let refreshError: string | null = null;
      try {
        await onActionComplete?.();
      } catch (error) {
        refreshError = getErrorMessage(
          error,
          'Failed to refresh DAG runs after submitting the batch action.'
        );
      }

      onReplaceSelection(
        results
          .filter((result) => !result.ok)
          .map((result) => ({
            name: result.name,
            dagRunId: result.dagRunId,
          }))
      );

      setProgress((previous) => ({
        ...previous,
        isRefreshing: false,
        refreshError,
      }));
    },
    [activeBatch, onActionComplete, onReplaceSelection, submitBatchItem]
  );

  return {
    activeBatch,
    closeDialog,
    isRunning,
    openBatchDialog,
    phase,
    progress,
    submitBatchAction,
  };
}
