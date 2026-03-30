import React from 'react';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { useErrorModal } from '@/components/ui/error-modal';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import ConfirmModal from '@/ui/ConfirmModal';
import { DAGRunSelectionItem } from '../../hooks/useBulkDAGRunSelection';

type BatchActionType = 'retry' | 'reschedule';

type BatchActionResponse = components['schemas']['DAGRunBatchActionResponse'];

interface DAGRunBatchActionsProps {
  onActionComplete?: () => Promise<void> | void;
  onClearSelection: () => void;
  onSelectAllVisible: () => void;
  selectedRuns: DAGRunSelectionItem[];
  visibleCount: number;
}

const actionLabels: Record<BatchActionType, string> = {
  retry: 'Retry selected',
  reschedule: 'Reschedule selected',
};

const actionNouns: Record<BatchActionType, string> = {
  retry: 'retry',
  reschedule: 'reschedule',
};

const actionPastTense: Record<BatchActionType, string> = {
  retry: 'Retried',
  reschedule: 'Rescheduled',
};

function DAGRunBatchActions({
  onActionComplete,
  onClearSelection,
  onSelectAllVisible,
  selectedRuns,
  visibleCount,
}: DAGRunBatchActionsProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const { showError } = useErrorModal();
  const { showToast } = useSimpleToast();
  const [pendingAction, setPendingAction] =
    React.useState<BatchActionType | null>(null);
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [resultsState, setResultsState] = React.useState<{
    action: BatchActionType;
    response: BatchActionResponse;
  } | null>(null);

  const selectedCount = selectedRuns.length;

  const submitBatchAction = React.useCallback(async () => {
    if (!pendingAction || selectedCount === 0) {
      setPendingAction(null);
      return;
    }

    const action = pendingAction;
    setPendingAction(null);
    setIsSubmitting(true);

    const path =
      action === 'retry' ? '/dag-runs/retry-batch' : '/dag-runs/reschedule-batch';

    const { data, error } = await client.POST(path, {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
      body: {
        items: selectedRuns.map((dagRun) => ({
          name: dagRun.name,
          dagRunId: dagRun.dagRunId,
        })),
      },
    });

    setIsSubmitting(false);

    if (error) {
      showError(
        error.message ||
          `Failed to ${actionNouns[action]} ${selectedCount} DAG run${selectedCount === 1 ? '' : 's'}`,
        'Refresh the page and try again.'
      );
      return;
    }

    if (!data) {
      showError(
        `Failed to ${actionNouns[action]} DAG runs`,
        'The server returned an empty response.'
      );
      return;
    }

    await Promise.resolve(onActionComplete?.());
    onClearSelection();

    if (data.failureCount === 0) {
      showToast(
        `${actionPastTense[action]} ${data.successCount} DAG run${data.successCount === 1 ? '' : 's'}`
      );
      return;
    }

    setResultsState({
      action,
      response: data,
    });
  }, [
    appBarContext.selectedRemoteNode,
    client,
    onActionComplete,
    onClearSelection,
    pendingAction,
    selectedCount,
    selectedRuns,
    showError,
    showToast,
  ]);

  const summaryText =
    selectedCount === 0
      ? `${visibleCount} visible`
      : `${selectedCount} selected of ${visibleCount} visible`;

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-card px-3 py-2">
        <div className="text-sm text-muted-foreground">{summaryText}</div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={onSelectAllVisible}
            disabled={visibleCount === 0 || isSubmitting}
          >
            Select all visible
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={onClearSelection}
            disabled={selectedCount === 0 || isSubmitting}
          >
            Clear selection
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setPendingAction('retry')}
            disabled={selectedCount === 0 || isSubmitting}
          >
            Retry selected
          </Button>
          <Button
            size="sm"
            onClick={() => setPendingAction('reschedule')}
            disabled={selectedCount === 0 || isSubmitting}
          >
            Reschedule selected
          </Button>
        </div>
      </div>

      <ConfirmModal
        title={pendingAction ? actionLabels[pendingAction] : 'Batch action'}
        buttonText={pendingAction ? actionLabels[pendingAction] : 'Run'}
        visible={pendingAction !== null}
        dismissModal={() => setPendingAction(null)}
        onSubmit={submitBatchAction}
      >
        <div className="space-y-2">
          <p>
            Do you want to {pendingAction ? actionNouns[pendingAction] : 'run'}{' '}
            {selectedCount} selected DAG run
            {selectedCount === 1 ? '' : 's'}?
          </p>
          <div className="max-h-56 space-y-2 overflow-y-auto rounded-md border bg-muted/20 p-3">
            {selectedRuns.map((dagRun) => (
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
      </ConfirmModal>

      <Dialog
        open={resultsState !== null}
        onOpenChange={(open) => {
          if (!open) {
            setResultsState(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {resultsState
                ? `${actionLabels[resultsState.action]} results`
                : 'Batch results'}
            </DialogTitle>
            <DialogDescription>
              {resultsState
                ? `${resultsState.response.successCount} succeeded, ${resultsState.response.failureCount} failed.`
                : ''}
            </DialogDescription>
          </DialogHeader>

          <div className="max-h-[55vh] space-y-3 overflow-y-auto pr-1">
            {resultsState?.response.results.map((result, index) => (
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
                {result.newDagRunId && (
                  <div className="mt-2 text-sm">
                    New DAG run:{' '}
                    <span className="font-mono">{result.newDagRunId}</span>
                  </div>
                )}
                {typeof result.queued === 'boolean' && (
                  <div className="mt-1 text-sm text-muted-foreground">
                    {result.queued ? 'Queued for execution' : 'Started immediately'}
                  </div>
                )}
                {result.error && (
                  <div className="mt-2 text-sm text-error">{result.error}</div>
                )}
              </div>
            ))}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setResultsState(null)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export default DAGRunBatchActions;
