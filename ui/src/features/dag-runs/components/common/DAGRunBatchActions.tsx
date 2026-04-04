// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { CheckCircle2, RefreshCw } from 'lucide-react';
import React from 'react';
import { DAGRunSelectionItem } from '../../hooks/useBulkDAGRunSelection';
import {
  BatchActionResult,
  BatchActionType,
  useDAGRunBatchSubmission,
} from '../../hooks/useDAGRunBatchSubmission';

interface DAGRunBatchActionsProps {
  loadedCount: number;
  onActionComplete?: () => Promise<void>;
  onClearSelection: () => void;
  onReplaceSelection: (items: DAGRunSelectionItem[]) => void;
  onSelectAllLoaded: () => void;
  selectedRuns: DAGRunSelectionItem[];
}

const actionLabels: Record<BatchActionType, string> = {
  retry: 'Retry selected',
  reschedule: 'Reschedule selected',
};

const actionVerbs: Record<BatchActionType, string> = {
  retry: 'retry',
  reschedule: 'reschedule',
};

function DAGRunBatchActions({
  loadedCount,
  onActionComplete,
  onClearSelection,
  onReplaceSelection,
  onSelectAllLoaded,
  selectedRuns,
}: DAGRunBatchActionsProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const {
    activeBatch,
    closeDialog,
    isRunning,
    openBatchDialog,
    phase,
    progress,
    submitBatchAction,
  } = useDAGRunBatchSubmission({
    onActionComplete,
    onReplaceSelection,
    selectedRuns,
  });
  const selectedCount = selectedRuns.length;
  const snapshot = activeBatch?.snapshot ?? [];
  const totalCount = snapshot.length;
  const isLocked = phase === 'running' || progress.isRefreshing;
  const isProcessing = phase === 'running' || phase === 'complete';
  const [specFromFile, setSpecFromFile] = React.useState(false);
  const [useCurrentDagFile, setUseCurrentDagFile] = React.useState(false);
  const [rescheduleSourceLoading, setRescheduleSourceLoading] =
    React.useState(false);

  const summaryText =
    selectedCount === 0
      ? `${loadedCount} loaded`
      : `${selectedCount} selected of ${loadedCount} loaded`;

  React.useEffect(() => {
    if (phase !== 'confirm' || activeBatch?.action !== 'reschedule') {
      setSpecFromFile(false);
      setUseCurrentDagFile(false);
      setRescheduleSourceLoading(false);
      return;
    }

    let cancelled = false;
    setRescheduleSourceLoading(true);

    Promise.all(
      activeBatch.snapshot.map(async (dagRun) => {
        const { data } = await client.GET('/dag-runs/{name}/{dagRunId}', {
          params: {
            path: {
              name: dagRun.name,
              dagRunId: dagRun.dagRunId,
            },
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
          },
        });

        return Boolean(data?.dagRunDetails?.specFromFile);
      })
    )
      .then((results) => {
        if (cancelled) {
          return;
        }
        const available = results.length > 0 && results.every(Boolean);
        setSpecFromFile(available);
        setUseCurrentDagFile(available);
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setSpecFromFile(false);
        setUseCurrentDagFile(false);
      })
      .finally(() => {
        if (!cancelled) {
          setRescheduleSourceLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [activeBatch, appBarContext.selectedRemoteNode, client, phase]);

  const renderResultDetails = (
    action: BatchActionType,
    result: BatchActionResult
  ) => {
    if (!result.ok) {
      return <div className="mt-2 text-sm text-error">{result.error}</div>;
    }

    if (action === 'retry') {
      return (
        <div className="mt-2 text-sm text-muted-foreground">
          Retry request accepted
        </div>
      );
    }

    return (
      <div className="mt-2 space-y-1 text-sm">
        {result.newDagRunId ? (
          <div>
            New DAG run: <span className="font-mono">{result.newDagRunId}</span>
          </div>
        ) : (
          <div className="text-muted-foreground">
            Reschedule request accepted
          </div>
        )}
        {typeof result.queued === 'boolean' && (
          <div className="text-muted-foreground">
            {result.queued ? 'Queued for execution' : 'Started immediately'}
          </div>
        )}
      </div>
    );
  };

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-card px-3 py-2">
        <div className="text-sm text-muted-foreground">{summaryText}</div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={onSelectAllLoaded}
            disabled={loadedCount === 0 || isRunning}
          >
            Select all loaded
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={onClearSelection}
            disabled={selectedCount === 0 || isRunning}
          >
            Clear selection
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => openBatchDialog('retry')}
            disabled={selectedCount === 0 || isRunning}
          >
            Retry selected
          </Button>
          <Button
            size="sm"
            onClick={() => openBatchDialog('reschedule')}
            disabled={selectedCount === 0 || isRunning}
          >
            Reschedule selected
          </Button>
        </div>
      </div>

      <Dialog
        open={phase !== null}
        onOpenChange={(open) => {
          if (!open) {
            closeDialog();
          }
        }}
      >
        <DialogContent
          hideCloseButton
          className="sm:max-w-2xl"
          onPointerDownOutside={(event) => {
            if (isLocked) {
              event.preventDefault();
            }
          }}
          onEscapeKeyDown={(event) => {
            if (isLocked) {
              event.preventDefault();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>
              {activeBatch ? actionLabels[activeBatch.action] : 'Batch action'}
            </DialogTitle>
            <DialogDescription>
              {phase === 'confirm' && activeBatch
                ? `Submit ${activeBatch.snapshot.length} ${actionVerbs[activeBatch.action]} request${activeBatch.snapshot.length === 1 ? '' : 's'} using the existing DAG-run API.`
                : isProcessing
                  ? `Processing ${totalCount} request${totalCount === 1 ? '' : 's'} using the existing DAG-run API.`
                  : ''}
            </DialogDescription>
          </DialogHeader>

          {phase === 'confirm' && activeBatch && (
            <div className="space-y-3">
              <p className="text-sm text-foreground">
                Do you want to {actionVerbs[activeBatch.action]}{' '}
                {activeBatch.snapshot.length} selected DAG run
                {activeBatch.snapshot.length === 1 ? '' : 's'}?
              </p>
              <div className="max-h-56 space-y-2 overflow-y-auto rounded-md border bg-muted/20 p-3">
                {activeBatch.snapshot.map((dagRun) => (
                  <div
                    key={`${dagRun.name}-${dagRun.dagRunId}`}
                    className="text-sm"
                  >
                    <div className="font-medium">{dagRun.name}</div>
                    <div className="font-mono text-xs text-muted-foreground">
                      {dagRun.dagRunId}
                    </div>
                  </div>
                ))}
              </div>
              {activeBatch.action === 'reschedule' && (
                <div
                  role="button"
                  tabIndex={rescheduleSourceLoading || !specFromFile ? -1 : 0}
                  aria-disabled={rescheduleSourceLoading || !specFromFile}
                  onClick={() => {
                    if (rescheduleSourceLoading || !specFromFile) {
                      return;
                    }
                    setUseCurrentDagFile((value) => !value);
                  }}
                  onKeyDown={(event) => {
                    if (
                      rescheduleSourceLoading ||
                      !specFromFile ||
                      (event.key !== 'Enter' && event.key !== ' ')
                    ) {
                      return;
                    }
                    event.preventDefault();
                    setUseCurrentDagFile((value) => !value);
                  }}
                  className="flex w-full items-start gap-3 rounded-md border px-3 py-3 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring aria-disabled:cursor-not-allowed aria-disabled:opacity-70 aria-disabled:hover:bg-transparent"
                >
                  <Checkbox
                    id="use-current-dag-file-batch"
                    aria-label="Use original DAG file"
                    checked={useCurrentDagFile}
                    disabled={rescheduleSourceLoading || !specFromFile}
                    onCheckedChange={(checked) =>
                      setUseCurrentDagFile(checked as boolean)
                    }
                    className="mt-0.5 h-5 w-5 border-border pointer-events-none"
                  />
                  <div className="space-y-0.5">
                    <Label
                      htmlFor="use-current-dag-file-batch"
                      className="cursor-pointer text-sm font-medium"
                    >
                      Use original DAG file
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      {rescheduleSourceLoading
                        ? 'Checking whether the selected DAG runs still have their original DAG files.'
                        : specFromFile
                          ? 'Use the current spec from the original DAG file for every selected DAG run.'
                          : 'Stored YAML snapshots will be used because one or more selected DAG runs do not have the original DAG file available.'}
                    </p>
                  </div>
                </div>
              )}
            </div>
          )}

          {isProcessing && activeBatch && (
            <div className="space-y-4">
              <div className="rounded-md border bg-muted/20 p-4">
                <div className="flex items-start gap-3">
                  {isLocked ? (
                    <RefreshCw className="mt-0.5 h-5 w-5 animate-spin text-muted-foreground" />
                  ) : (
                    <CheckCircle2 className="mt-0.5 h-5 w-5 text-success" />
                  )}
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="text-sm font-medium text-foreground">
                      {phase === 'running'
                        ? 'Submitting requests...'
                        : progress.isRefreshing
                          ? 'Refreshing DAG runs...'
                          : 'Finished submitting requests'}
                    </div>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span className="font-mono tabular-nums">
                        {progress.processedCount}/{totalCount} processed
                      </span>
                      <span>{progress.successCount} succeeded</span>
                      <span>{progress.failureCount} failed</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="rounded-md border bg-muted/20 p-3">
                <div className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Current item
                </div>
                {progress.currentItem ? (
                  <>
                    <div className="font-medium">
                      {progress.currentItem.name}
                    </div>
                    <div className="font-mono text-xs text-muted-foreground">
                      {progress.currentItem.dagRunId}
                    </div>
                  </>
                ) : (
                  <div className="text-sm text-muted-foreground">
                    {progress.isRefreshing
                      ? 'Refreshing the DAG-run list'
                      : 'All requests have been submitted'}
                  </div>
                )}
              </div>

              {progress.refreshError && (
                <div className="rounded-md border border-error/30 bg-error-muted p-3 text-sm text-error">
                  {progress.refreshError}
                </div>
              )}

              <div className="rounded-md border">
                <div className="border-b px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Results
                </div>
                <div className="min-h-40 max-h-[45vh] space-y-3 overflow-y-auto p-3">
                  {progress.results.length === 0 ? (
                    <div className="flex min-h-32 items-center justify-center text-sm text-muted-foreground">
                      Results will appear here as each request finishes.
                    </div>
                  ) : (
                    progress.results.map((result, index) => (
                      <div
                        key={`${result.name}-${result.dagRunId}-${index}`}
                        data-testid="batch-action-result-item"
                        className="rounded-md border p-3"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="font-medium">{result.name}</div>
                            <div className="font-mono text-xs text-muted-foreground">
                              {result.dagRunId}
                            </div>
                          </div>
                          <div
                            className={`text-xs font-medium ${result.ok ? 'text-success' : 'text-error'}`}
                          >
                            {result.ok ? 'Succeeded' : 'Failed'}
                          </div>
                        </div>
                        {renderResultDetails(activeBatch.action, result)}
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            {phase === 'confirm' && activeBatch && (
              <>
                <Button variant="outline" onClick={closeDialog}>
                  Cancel
                </Button>
                <Button
                  onClick={() =>
                    submitBatchAction(
                      activeBatch.action === 'reschedule'
                        ? { useCurrentDagFile }
                        : undefined
                    )
                  }
                >
                  {activeBatch.action === 'retry'
                    ? `Retry ${activeBatch.snapshot.length} Run${activeBatch.snapshot.length === 1 ? '' : 's'}`
                    : `Reschedule ${activeBatch.snapshot.length} Run${activeBatch.snapshot.length === 1 ? '' : 's'}`}
                </Button>
              </>
            )}
            {(phase === 'running' || progress.isRefreshing) && (
              <Button disabled>
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                {phase === 'running' ? 'Submitting...' : 'Refreshing...'}
              </Button>
            )}
            {phase === 'complete' && !progress.isRefreshing && (
              <Button variant="outline" onClick={closeDialog}>
                Close
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export default DAGRunBatchActions;
