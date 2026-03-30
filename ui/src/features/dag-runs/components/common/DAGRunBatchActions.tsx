import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { CheckCircle2, RefreshCw } from 'lucide-react';
import React from 'react';
import { DAGRunSelectionItem } from '../../hooks/useBulkDAGRunSelection';
import {
  BatchActionResult,
  BatchActionType,
  useDAGRunBatchSubmission,
} from '../../hooks/useDAGRunBatchSubmission';

interface DAGRunBatchActionsProps {
  matchingCount: number;
  onActionComplete?: () => Promise<void>;
  onClearSelection: () => void;
  onReplaceSelection: (items: DAGRunSelectionItem[]) => void;
  onSelectAllMatching: () => void;
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
  matchingCount,
  onActionComplete,
  onClearSelection,
  onReplaceSelection,
  onSelectAllMatching,
  selectedRuns,
}: DAGRunBatchActionsProps) {
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

  const summaryText =
    selectedCount === 0
      ? `${matchingCount} matching`
      : `${selectedCount} selected of ${matchingCount} matching`;

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
            onClick={onSelectAllMatching}
            disabled={matchingCount === 0 || isRunning}
          >
            Select all matching
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
                <Button onClick={submitBatchAction}>
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
