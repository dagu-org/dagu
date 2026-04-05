// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import React from 'react';
import useSWR from 'swr';
import type { components } from '../../../api/v1/schema';
import { PathsQueuesNameItemsGetParametersQueryType } from '../../../api/v1/schema';
import { Button } from '../../../components/ui/button';
import { Checkbox } from '../../../components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../../components/ui/dialog';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useConfig } from '../../../contexts/ConfigContext';
import { useClient } from '../../../hooks/api';
import dayjs from '../../../lib/dayjs';
import { cn } from '../../../lib/utils';
import DAGPagination from '../../dags/components/common/DAGPagination';
import {
  QueueBatchResult,
  useQueueBatchDequeue,
} from '../hooks/useQueueBatchDequeue';
import { useQueueSelection } from '../hooks/useQueueSelection';
import StatusChip from '../../../ui/StatusChip';

interface QueueCardProps {
  queue: components['schemas']['Queue'];
  isSelected?: boolean;
  onDAGRunClick: (dagRun: components['schemas']['DAGRunSummary']) => void;
  onQueueChanged?: () => void | Promise<void>;
}

function QueueCard({
  queue,
  isSelected,
  onDAGRunClick,
  onQueueChanged,
}: QueueCardProps) {
  const config = useConfig();
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const [isExpanded, setIsExpanded] = React.useState(true);
  const [queuedPage, setQueuedPage] = React.useState(1);
  const [perPage, setPerPage] = React.useState(10);

  const remoteNode = appBarContext?.selectedRemoteNode || 'local';
  React.useEffect(() => {
    setQueuedPage(1);
  }, [remoteNode, queue.name, perPage]);

  const toggleExpanded = () => setIsExpanded(!isExpanded);

  const shouldFetchQueued = isExpanded && queue.queuedCount > 0;
  const {
    data: queuedResponse,
    mutate: mutateQueuedData,
    isLoading,
  } = useSWR(
    shouldFetchQueued
      ? ['listQueueItems', queue.name, queuedPage, perPage, remoteNode]
      : null,
    async () => {
      const response = await client.GET('/queues/{name}/items', {
        params: {
          path: { name: queue.name },
          query: {
            type: PathsQueuesNameItemsGetParametersQueryType.queued,
            page: queuedPage,
            perPage,
            remoteNode,
          },
        },
      });
      return response.data;
    },
    {
      refreshInterval: 3000,
      keepPreviousData: true,
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  );

  const queuedItems = queuedResponse?.items ?? [];
  const pagination = queuedResponse?.pagination;
  const loadedCount = queuedItems.length;

  const {
    clearSelection,
    isSelected: isQueueItemSelected,
    replaceSelection,
    selectAllLoaded,
    selectedCount,
    selectedRuns,
    toggleSelection,
  } = useQueueSelection(queuedItems);

  const {
    activeBatch,
    closeDialog,
    isRunning,
    openBatchDialog,
    phase,
    progress,
    submitBatchDequeue,
  } = useQueueBatchDequeue({
    onActionComplete: async () => {
      await mutateQueuedData();
      if (onQueueChanged) {
        await onQueueChanged();
      }
    },
    onReplaceSelection: replaceSelection,
    selectedRuns,
  });

  const utilization = React.useMemo(() => {
    if (queue.type !== 'global' || !queue.maxConcurrency) {
      return null;
    }
    const running = queue.runningCount || 0;
    return Math.round((running / queue.maxConcurrency) * 100);
  }, [queue]);

  const selectedSummaryText =
    selectedCount === 0
      ? `${loadedCount} loaded`
      : `${selectedCount} selected of ${loadedCount} loaded`;
  const headerCheckboxState =
    loadedCount === 0
      ? false
      : selectedCount === loadedCount
        ? true
        : selectedCount > 0
          ? 'indeterminate'
          : false;
  const totalBatchCount = activeBatch?.snapshot.length ?? 0;
  const isLocked = phase === 'running' || progress.isRefreshing;
  const isProcessing = phase === 'running' || phase === 'complete';

  function formatDateTime(datetime: string | undefined): string {
    if (!datetime) {
      return 'N/A';
    }
    const date = dayjs(datetime);
    const offset = config.tzOffsetInSec;
    const format = 'MMM D, HH:mm:ss';
    return offset !== undefined
      ? date.utcOffset(offset / 60).format(format)
      : date.format(format);
  }

  function renderResultDetails(result: QueueBatchResult): React.JSX.Element {
    if (!result.ok) {
      return <div className="mt-2 text-sm text-error">{result.error}</div>;
    }

    return (
      <div className="mt-2 text-sm text-muted-foreground">
        {result.message ?? 'Dequeue request accepted.'}
      </div>
    );
  }

  function DAGRunRow({
    dagRun,
    selectable = false,
    showQueuedAt = false,
  }: {
    dagRun: components['schemas']['DAGRunSummary'];
    selectable?: boolean;
    showQueuedAt?: boolean;
  }): React.JSX.Element {
    const selected = selectable && isQueueItemSelected(dagRun);

    return (
      <tr
        onClick={() => onDAGRunClick(dagRun)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onDAGRunClick(dagRun);
          }
        }}
        role="button"
        tabIndex={0}
        className={cn(
          'cursor-pointer transition-colors focus:bg-muted/50 focus:outline-none hover:bg-muted/30',
          selected && 'bg-muted/20'
        )}
      >
        {selectable && (
          <td
            className="w-10 py-1.5 px-2 align-middle"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            <div className="flex h-8 w-8 items-center justify-center">
              <Checkbox
                aria-label={`Select ${dagRun.name} ${dagRun.dagRunId}`}
                checked={selected}
                onCheckedChange={() =>
                  toggleSelection({
                    name: dagRun.name,
                    dagRunId: dagRun.dagRunId,
                  })
                }
              />
            </div>
          </td>
        )}
        <td className="py-1.5 px-2 text-xs font-medium">{dagRun.name}</td>
        <td className="py-1.5 px-2">
          <StatusChip status={dagRun.status} size="xs">
            {dagRun.statusLabel}
          </StatusChip>
        </td>
        <td className="py-1.5 px-2 text-xs text-muted-foreground tabular-nums">
          <div className="flex flex-col gap-0.5">
            {dagRun.scheduleTime && (
              <span>
                <span className="text-muted-foreground/80">Scheduled </span>
                {formatDateTime(dagRun.scheduleTime)}
              </span>
            )}
            <span>
              <span className="text-muted-foreground/80">
                {showQueuedAt ? 'Queued ' : 'Started '}
              </span>
              {formatDateTime(showQueuedAt ? dagRun.queuedAt : dagRun.startedAt)}
            </span>
          </div>
        </td>
        <td className="py-1.5 px-2 text-xs text-muted-foreground font-mono">
          {dagRun.dagRunId}
        </td>
      </tr>
    );
  }

  return (
    <>
      <div
        className={cn(
          'card-obsidian transition-all duration-300 dark:hover:bg-white/[0.05] dark:hover:border-white/10',
          isSelected && 'shadow-[0_0_20px_rgba(var(--primary-rgb),0.1)]'
        )}
      >
        <div
          className="px-3 py-2 cursor-pointer hover:bg-muted/30 transition-colors"
          onClick={toggleExpanded}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2">
                {isExpanded ? (
                  <ChevronDown className="h-4 w-4 text-muted-foreground" />
                ) : (
                  <ChevronRight className="h-4 w-4 text-muted-foreground" />
                )}
                <span className="font-medium text-sm">{queue.name}</span>
                <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                  {queue.type}
                </span>
              </div>

              {queue.type === 'global' && queue.maxConcurrency && (
                <div className="flex items-center gap-2">
                  <div className="w-12 h-1 bg-muted rounded-full overflow-hidden">
                    <div
                      className="h-full transition-all duration-300 bg-foreground/40"
                      style={{ width: `${utilization || 0}%` }}
                    />
                  </div>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {queue.runningCount || 0}/{queue.maxConcurrency}
                  </span>
                </div>
              )}
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground">
              <div className="flex items-baseline gap-1">
                <span className="text-sm font-light tabular-nums text-foreground">
                  {queue.runningCount || 0}
                </span>
                <span>running</span>
              </div>
              <div className="flex items-baseline gap-1">
                <span
                  className={cn(
                    'text-sm font-light tabular-nums',
                    (queue.queuedCount || 0) > 0
                      ? 'text-foreground'
                      : 'text-muted-foreground/50'
                  )}
                >
                  {queue.queuedCount || 0}
                </span>
                <span>queued</span>
              </div>
              {utilization !== null && (
                <div className="flex items-baseline gap-1">
                  <span className="text-sm font-light tabular-nums text-foreground">
                    {utilization}%
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>

        {isExpanded && (
          <div className="border-t">
            {queue.running && queue.running.length > 0 && (
              <div>
                <div className="px-3 py-2 bg-muted/10">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                      Running ({queue.running.length})
                    </span>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b border-border">
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            DAG
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Status
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Timing
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Run ID
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border/50">
                        {queue.running.map((dagRun) => (
                          <DAGRunRow key={dagRun.dagRunId} dagRun={dagRun} />
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}

            {queue.queuedCount > 0 && (
              <div
                className={
                  queue.running && queue.running.length > 0 ? 'border-t' : ''
                }
              >
                <div className="px-3 py-2 bg-muted/10">
                  <div className="mb-2 flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
                    <div className="flex flex-wrap items-center gap-3">
                      <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Queued ({queue.queuedCount})
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {selectedSummaryText}
                      </span>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={(event) => {
                          event.stopPropagation();
                          selectAllLoaded();
                        }}
                        disabled={loadedCount === 0 || isRunning}
                      >
                        Select all loaded
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={(event) => {
                          event.stopPropagation();
                          clearSelection();
                        }}
                        disabled={selectedCount === 0 || isRunning}
                      >
                        Clear selection
                      </Button>
                      <Button
                        size="sm"
                        onClick={(event) => {
                          event.stopPropagation();
                          openBatchDialog();
                        }}
                        disabled={selectedCount === 0 || isRunning}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        <span className="ml-1">Dequeue selected</span>
                      </Button>
                    </div>
                  </div>
                  <div
                    className={cn(
                      'overflow-x-auto transition-opacity',
                      isLoading && 'opacity-70'
                    )}
                  >
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b border-border">
                          <th className="w-10 py-1 px-2 align-middle">
                            <div className="flex h-8 w-8 items-center justify-center">
                              <Checkbox
                                aria-label={`Select all loaded queue items for ${queue.name}`}
                                checked={headerCheckboxState}
                                disabled={loadedCount === 0 || isRunning}
                                onCheckedChange={(checked) => {
                                  if (checked) {
                                    selectAllLoaded();
                                    return;
                                  }
                                  clearSelection();
                                }}
                              />
                            </div>
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            DAG
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Status
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Timing
                          </th>
                          <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                            Run ID
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border/50">
                        {queuedItems.map((dagRun) => (
                          <DAGRunRow
                            key={dagRun.dagRunId}
                            dagRun={dagRun}
                            selectable
                            showQueuedAt
                          />
                        ))}
                      </tbody>
                    </table>
                  </div>
                  {pagination && pagination.totalRecords > 0 && (
                    <div className="flex items-center justify-between pt-2 border-t mt-2">
                      <DAGPagination
                        totalPages={pagination.totalPages}
                        page={pagination.currentPage}
                        pageChange={setQueuedPage}
                        pageLimit={perPage}
                        onPageLimitChange={setPerPage}
                      />
                    </div>
                  )}
                </div>
              </div>
            )}

            {(!queue.running || queue.running.length === 0) &&
              queue.queuedCount === 0 && (
                <div className="px-3 py-4 text-center text-muted-foreground text-xs">
                  No DAGs running or queued
                </div>
              )}
          </div>
        )}
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
            <DialogTitle>Dequeue selected</DialogTitle>
            <DialogDescription>
              {phase === 'confirm'
                ? `Submit ${totalBatchCount} dequeue request${totalBatchCount === 1 ? '' : 's'} using the existing DAG-run API.`
                : isProcessing
                  ? `Processing ${totalBatchCount} dequeue request${totalBatchCount === 1 ? '' : 's'} using the existing DAG-run API.`
                  : ''}
            </DialogDescription>
          </DialogHeader>

          {phase === 'confirm' && activeBatch && (
            <div className="space-y-3">
              <p className="text-sm text-foreground">
                Do you want to dequeue {activeBatch.snapshot.length} selected
                DAG run{activeBatch.snapshot.length === 1 ? '' : 's'} from{' '}
                {queue.name}?
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

          {isProcessing && (
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
                        ? 'Submitting dequeue requests...'
                        : progress.isRefreshing
                          ? 'Refreshing queue...'
                          : 'Finished submitting dequeue requests'}
                    </div>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span className="font-mono tabular-nums">
                        {progress.processedCount}/{totalBatchCount} processed
                      </span>
                      <span>{progress.successCount} succeeded</span>
                      <span>{progress.skippedCount} skipped</span>
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
                      ? 'Refreshing queue data'
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
                            className={cn(
                              'text-xs font-medium',
                              result.skipped
                                ? 'text-muted-foreground'
                                : result.ok
                                  ? 'text-success'
                                  : 'text-error'
                            )}
                          >
                            {result.skipped
                              ? 'Skipped'
                              : result.ok
                                ? 'Succeeded'
                                : 'Failed'}
                          </div>
                        </div>
                        {renderResultDetails(result)}
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
                <Button onClick={submitBatchDequeue}>
                  Dequeue {activeBatch.snapshot.length} run
                  {activeBatch.snapshot.length === 1 ? '' : 's'}
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

export default QueueCard;
