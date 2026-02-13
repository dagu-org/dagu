import {
  components,
  SyncDAGKind,
  SyncStatus,
  SyncSummary,
} from '@/api/v1/schema';
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
import { useErrorModal } from '@/components/ui/error-modal';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useSimpleToast } from '@/components/ui/simple-toast';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useCanWrite } from '@/contexts/AuthContext';
import { useClient, useQuery } from '@/hooks/api';
import dayjs from '@/lib/dayjs';
import { cn } from '@/lib/utils';
import ConfirmModal from '@/ui/ConfirmModal';
import {
  Download,
  FileCode,
  GitBranch,
  RefreshCw,
  Settings,
  Trash2,
  Upload,
} from 'lucide-react';
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { DiffModal } from './DiffModal';

type SyncStatusResponse = components['schemas']['SyncStatusResponse'];
type SyncConfigResponse = components['schemas']['SyncConfigResponse'];
type SyncDAGDiffResponse = components['schemas']['SyncDAGDiffResponse'];
type SyncDAGState = components['schemas']['SyncDAGState'];
type StatusFilter = 'all' | 'modified' | 'untracked' | 'conflict';
type TypeFilter = 'all' | 'dag' | 'memory';
type SyncItemKind = 'dag' | 'memory';
type SyncRow = { dagId: string; dag: SyncDAGState; kind: SyncItemKind };

const statusFilters: StatusFilter[] = [
  'all',
  'modified',
  'untracked',
  'conflict',
];
const typeFilters: TypeFilter[] = ['all', 'dag', 'memory'];

function parseStatusFilter(value: string | null): StatusFilter {
  if (
    value === 'all' ||
    value === 'modified' ||
    value === 'untracked' ||
    value === 'conflict'
  ) {
    return value;
  }
  return 'all';
}

function parseTypeFilter(value: string | null): TypeFilter {
  if (value === 'all' || value === 'dag' || value === 'memory') {
    return value;
  }
  return 'all';
}

function normalizeSyncItemKind(dagId: string, dag: SyncDAGState): SyncItemKind {
  const rawKind = (dag as { kind?: SyncDAGKind | string }).kind;
  if (rawKind === SyncDAGKind.memory || rawKind === 'memory') {
    return 'memory';
  }
  if (rawKind === SyncDAGKind.dag || rawKind === 'dag') {
    return 'dag';
  }
  return dagId.startsWith('memory/') ? 'memory' : 'dag';
}

function formatSyncDisplayName(dagId: string, kind: SyncItemKind): string {
  const extension = kind === 'memory' ? '.md' : '.yaml';
  return `${dagId}${extension}`;
}

// Subtle, readable status colors
const summaryConfig: Record<
  SyncSummary,
  { label: string; badgeClass: string }
> = {
  [SyncSummary.synced]: {
    label: 'Synced',
    badgeClass: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  },
  [SyncSummary.pending]: {
    label: 'Unpublished',
    badgeClass: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
  },
  [SyncSummary.conflict]: {
    label: 'Conflict',
    badgeClass: 'bg-rose-500/10 text-rose-700 dark:text-rose-400',
  },
  [SyncSummary.error]: {
    label: 'Error',
    badgeClass: 'bg-rose-500/10 text-rose-700 dark:text-rose-400',
  },
};

// Status configuration for dots
const statusConfig: Record<SyncStatus, { color: string; label: string }> = {
  [SyncStatus.synced]: { color: 'bg-emerald-500', label: 'synced' },
  [SyncStatus.modified]: { color: 'bg-amber-500', label: 'modified' },
  [SyncStatus.untracked]: { color: 'bg-blue-500', label: 'untracked' },
  [SyncStatus.conflict]: { color: 'bg-rose-500', label: 'conflict' },
};

function StatusDot({ status }: { status: SyncStatus }) {
  const config = statusConfig[status];
  return (
    <div className="flex items-center gap-1.5">
      <span className={cn('h-2 w-2 rounded-full', config.color)} />
      <span className="text-xs text-muted-foreground">{config.label}</span>
    </div>
  );
}

export default function GitSyncPage() {
  const appBarContext = useContext(AppBarContext);
  const { setTitle } = appBarContext;
  const client = useClient();
  const canWrite = useCanWrite();
  const { showToast } = useSimpleToast();
  const { showError } = useErrorModal();

  // State
  const [isPulling, setIsPulling] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [publishModal, setPublishModal] = useState<{
    open: boolean;
    dagId?: string;
  }>({ open: false });
  const [commitMessage, setCommitMessage] = useState('');
  const [diffModal, setDiffModal] = useState<{ open: boolean; dagId?: string }>(
    { open: false }
  );
  const [diffData, setDiffData] = useState<SyncDAGDiffResponse | null>(null);
  const [revertModal, setRevertModal] = useState<{
    open: boolean;
    dagId?: string;
  }>({ open: false });
  const [selectedDags, setSelectedDags] = useState<Set<string>>(new Set());
  const userTouchedSelectionRef = useRef(false);
  const prevPublishableRef = useRef<string>('');
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    setTitle('Git Sync');
  }, [setTitle]);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Fetch sync status
  const { data: statusData, mutate: mutateStatus } = useQuery(
    '/sync/status',
    {
      params: {
        query: { remoteNode },
      },
    },
    { refreshInterval: 5000 }
  );

  // Fetch sync config
  const { data: configData } = useQuery('/sync/config', {
    params: {
      query: { remoteNode },
    },
  });

  const status = statusData as SyncStatusResponse | undefined;
  const config = configData as SyncConfigResponse | undefined;
  const statusFilter = parseStatusFilter(searchParams.get('status'));
  const typeFilter = parseTypeFilter(searchParams.get('type'));

  const setFilters = useCallback(
    (next: Partial<{ status: StatusFilter; type: TypeFilter }>) => {
      const nextStatus = next.status ?? statusFilter;
      const nextType = next.type ?? typeFilter;
      const params = new URLSearchParams(searchParams);

      if (nextStatus === 'all') {
        params.delete('status');
      } else {
        params.set('status', nextStatus);
      }

      if (nextType === 'all') {
        params.delete('type');
      } else {
        params.set('type', nextType);
      }

      setSearchParams(params, { replace: true });
    },
    [searchParams, setSearchParams, statusFilter, typeFilter]
  );

  const syncRows = useMemo<SyncRow[]>(
    () =>
      status?.dags
        ? Object.entries(status.dags).map(([dagId, dag]) => ({
            dagId,
            dag,
            kind: normalizeSyncItemKind(dagId, dag),
          }))
        : [],
    [status?.dags]
  );

  const publishableKey = useMemo(() => {
    return syncRows
      .filter(
        ({ dag }) =>
          dag.status === SyncStatus.modified ||
          dag.status === SyncStatus.untracked
      )
      .map(({ dagId }) => dagId)
      .sort()
      .join(',');
  }, [syncRows]);

  useEffect(() => {
    userTouchedSelectionRef.current = false;
    prevPublishableRef.current = '';
    setSelectedDags(new Set());
  }, [remoteNode]);

  // Auto-select publishable DAGs without overriding user manual choices on polling.
  useEffect(() => {
    const next = publishableKey ? publishableKey.split(',') : [];
    const prev = prevPublishableRef.current
      ? prevPublishableRef.current.split(',')
      : [];
    const prevSet = new Set(prev);
    const nextSet = new Set(next);

    setSelectedDags((current) => {
      if (!userTouchedSelectionRef.current) {
        return new Set(next);
      }

      const updated = new Set<string>();
      for (const id of current) {
        if (nextSet.has(id)) {
          updated.add(id);
        }
      }
      for (const id of next) {
        if (!prevSet.has(id)) {
          updated.add(id)
        }
      }
      return updated;
    });

    prevPublishableRef.current = publishableKey;
  }, [publishableKey]);

  // Handlers
  const handlePull = async () => {
    setIsPulling(true);
    try {
      const response = await client.POST('/sync/pull', {
        params: { query: { remoteNode } },
      });
      if (response.error) {
        showError(response.error.message || 'Pull failed');
      } else {
        showToast(`Pulled ${response.data?.synced?.length || 0} DAGs`);
        mutateStatus();
      }
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Pull failed');
    } finally {
      setIsPulling(false);
    }
  };

  const handlePublish = async (dagId?: string, force?: boolean) => {
    setIsPublishing(true);
    try {
      const response = dagId
        ? await client.POST('/dags/{name}/publish', {
            params: { path: { name: dagId }, query: { remoteNode } },
            body: {
              message: commitMessage || `Update ${dagId}`,
              force: force || false,
            },
          })
        : await client.POST('/sync/publish-all', {
            params: { query: { remoteNode } },
            body: {
              message: commitMessage || 'Batch update',
              dagIds: Array.from(selectedDags),
            },
          });

      if (response.error) {
        showError(response.error.message || 'Publish failed');
      } else {
        const count = dagId
          ? dagId
          : `${response.data?.synced?.length || 0} DAGs`;
        showToast(`Published ${count}`);
        setPublishModal({ open: false });
        setDiffModal({ open: false });
        setCommitMessage('');
        setSelectedDags(new Set());
        userTouchedSelectionRef.current = false;
        mutateStatus();
      }
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Publish failed');
    } finally {
      setIsPublishing(false);
    }
  };

  const handleDiscard = async (dagId: string) => {
    try {
      const response = await client.POST('/dags/{name}/discard', {
        params: { path: { name: dagId }, query: { remoteNode } },
      });
      if (response.error) {
        showError(response.error.message || 'Discard failed');
      } else {
        showToast(`Discarded changes to ${dagId}`);
        mutateStatus();
      }
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Discard failed');
    }
  };

  const handleTestConnection = async () => {
    try {
      const response = await client.POST('/sync/test-connection', {
        params: { query: { remoteNode } },
      });
      if (response.error) {
        showError(response.error.message || 'Connection test failed');
      } else if (response.data?.success) {
        showToast('Connection successful');
      } else {
        showError(response.data?.error || 'Connection test failed');
      }
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Connection test failed');
    }
  };

  const handleViewDiff = async (dagId: string) => {
    // Fetch data first, then open modal
    try {
      const response = await client.GET('/sync/dags/{name}/diff', {
        params: { path: { name: dagId }, query: { remoteNode } },
      });
      if (response.data) {
        setDiffData(response.data);
        setDiffModal({ open: true, dagId });
      }
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Failed to load diff');
    }
  };

  const filteredRows = useMemo(
    () =>
      syncRows.filter(({ dag, kind }) => {
        const typeMatches = typeFilter === 'all' || kind === typeFilter;
        const statusMatches =
          statusFilter === 'all' || dag.status === statusFilter;
        return typeMatches && statusMatches;
      }),
    [syncRows, typeFilter, statusFilter]
  );

  // Publishable DAG IDs among currently visible (filtered) rows
  const publishableDagIds = useMemo(
    () =>
      filteredRows
        .filter(
          ({ dag }) =>
            dag.status === SyncStatus.modified ||
            dag.status === SyncStatus.untracked
        )
        .map(({ dagId }) => dagId),
    [filteredRows]
  );

  const allPublishableSelected = useMemo(
    () =>
      publishableDagIds.length > 0 &&
      publishableDagIds.every((id) => selectedDags.has(id)),
    [publishableDagIds, selectedDags]
  );

  const handleToggleSelectAll = useCallback(() => {
    userTouchedSelectionRef.current = true;
    setSelectedDags((prev) => {
      const next = new Set(prev);
      if (allPublishableSelected) {
        for (const id of publishableDagIds) next.delete(id);
      } else {
        for (const id of publishableDagIds) next.add(id);
      }
      return next;
    });
  }, [allPublishableSelected, publishableDagIds]);

  const handleToggleDag = useCallback((dagId: string) => {
    userTouchedSelectionRef.current = true;
    setSelectedDags((prev) => {
      const next = new Set(prev);
      if (next.has(dagId)) next.delete(dagId);
      else next.add(dagId);
      return next;
    });
  }, []);

  const typeCounts = useMemo(() => {
    const counts: Record<TypeFilter, number> = {
      all: syncRows.length,
      dag: 0,
      memory: 0,
    };
    for (const { kind } of syncRows) {
      counts[kind] += 1;
    }
    return counts;
  }, [syncRows]);

  const statusCounts = useMemo(() => {
    const counts: Record<StatusFilter, number> = {
      all: 0,
      modified: 0,
      untracked: 0,
      conflict: 0,
    };

    for (const { dag, kind } of syncRows) {
      if (typeFilter !== 'all' && kind !== typeFilter) {
        continue;
      }
      counts.all += 1;
      if (dag.status === SyncStatus.modified) counts.modified += 1;
      if (dag.status === SyncStatus.untracked) counts.untracked += 1;
      if (dag.status === SyncStatus.conflict) counts.conflict += 1;
    }

    return counts;
  }, [syncRows, typeFilter]);

  const rowByID = useMemo(
    () => new Map(syncRows.map((row) => [row.dagId, row] as const)),
    [syncRows]
  );

  const selectedCounts = useMemo(() => {
    let dag = 0;
    let memory = 0;
    for (const dagID of selectedDags) {
      const row = rowByID.get(dagID);
      if (!row) continue;
      if (row.kind === 'memory') memory += 1;
      else dag += 1;
    }
    return { dag, memory, total: dag + memory };
  }, [selectedDags, rowByID]);

  const emptyStateMessage = useMemo(() => {
    if (typeFilter === 'all' && statusFilter === 'all') {
      return 'No items found';
    }
    if (typeFilter === 'all') {
      return `No ${statusFilter} items`;
    }
    const typeLabel = typeFilter === 'dag' ? 'DAG' : 'memory';
    if (statusFilter === 'all') {
      return `No ${typeLabel} items`;
    }
    return `No ${typeLabel} items with ${statusFilter} status`;
  }, [statusFilter, typeFilter]);

  // Not configured state
  if (!status?.enabled) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] text-center">
        <GitBranch className="h-16 w-16 text-muted-foreground/30 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Git Sync Not Configured</h2>
        <p className="text-muted-foreground max-w-md">
          Git sync allows you to synchronize DAG definitions with a remote Git
          repository. Configure it in your server settings.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 max-w-7xl pb-8">
      {/* Compact Header */}
      <div className="flex items-center justify-between py-2">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold">Git Sync</h1>
          <span className="text-xs text-muted-foreground font-mono">
            {status?.repository}:{status?.branch}
          </span>
          {status?.summary && (
            <span
              className={cn(
                'text-xs px-2 py-0.5 rounded',
                summaryConfig[status.summary]?.badgeClass
              )}
            >
              {summaryConfig[status.summary]?.label}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={handlePull}
            disabled={isPulling || !canWrite}
            title={!canWrite ? 'Write permission required' : 'Pull from remote'}
          >
            {isPulling ? (
              <RefreshCw className="h-4 w-4 animate-spin" />
            ) : (
              <Download className="h-4 w-4" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={() => setPublishModal({ open: true })}
            disabled={selectedDags.size === 0 || !config?.pushEnabled || !canWrite}
            title={
              !canWrite
                ? 'Write permission required'
                : !config?.pushEnabled
                  ? 'Push disabled in read-only mode'
                  : `Publish ${selectedDags.size} selected`
            }
          >
            <Upload className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={async () => {
              await mutateStatus();
              showToast('Status refreshed');
            }}
            title="Refresh status"
          >
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={() => setShowConfig(true)}
            title="Configuration"
          >
            <Settings className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Filter Controls */}
      <div className="flex items-center justify-between gap-2">
        <div className="inline-flex items-center rounded-md border border-border/60 bg-card p-0.5 text-xs">
          {typeFilters.map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setFilters({ type: f })}
              className={cn(
                'px-2.5 py-1 rounded transition-colors',
                typeFilter === f
                  ? 'bg-muted text-foreground'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {f === 'all' ? 'All' : f === 'dag' ? 'DAGs' : 'Memory'} ({typeCounts[f]})
            </button>
          ))}
        </div>
        {selectedCounts.total > 0 && (
          <span className="text-xs text-muted-foreground">
            Selected: {selectedCounts.dag} DAGs, {selectedCounts.memory} memory
          </span>
        )}
      </div>

      <div
        className="flex items-center gap-1 text-xs border-b border-border/40"
        role="tablist"
        aria-label="Filter items by status"
      >
        {statusFilters.map((f, index) => (
          <button
            key={f}
            role="tab"
            aria-selected={statusFilter === f}
            tabIndex={statusFilter === f ? 0 : -1}
            onClick={() => setFilters({ status: f })}
            onKeyDown={(e) => {
              if (e.key === 'ArrowRight') {
                e.preventDefault();
                const nextFilter = statusFilters[(index + 1) % statusFilters.length];
                if (nextFilter) setFilters({ status: nextFilter });
              } else if (e.key === 'ArrowLeft') {
                e.preventDefault();
                const prevFilter =
                  statusFilters[
                    (index - 1 + statusFilters.length) % statusFilters.length
                  ];
                if (prevFilter) setFilters({ status: prevFilter });
              }
            }}
            className={cn(
              'px-3 py-1.5 border-b-2 -mb-px transition-colors focus:outline-none',
              statusFilter === f
                ? 'border-foreground text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            {f.charAt(0).toUpperCase() + f.slice(1)} ({statusCounts[f]})
          </button>
        ))}
      </div>

      {/* DAGs Table */}
      <div className="bg-card border border-border rounded-md overflow-hidden shadow-sm">
        <Table className="text-xs">
          <TableHeader>
            <TableRow>
              <TableHead className="w-8">
                {publishableDagIds.length > 0 && (
                  <Checkbox
                    checked={allPublishableSelected}
                    onCheckedChange={handleToggleSelectAll}
                    aria-label="Select all publishable DAGs"
                  />
                )}
              </TableHead>
              <TableHead>DAG</TableHead>
              <TableHead className="w-24">Status</TableHead>
              <TableHead className="w-28">Synced</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredRows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className="text-center text-muted-foreground py-8 text-xs"
                >
                  {emptyStateMessage}
                </TableCell>
              </TableRow>
            ) : (
              filteredRows.map(({ dagId, dag, kind }) => (
                <TableRow
                  key={dagId}
                  className="h-9 cursor-pointer hover:bg-muted/50"
                  onClick={() => handleViewDiff(dagId)}
                >
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    {(dag.status === SyncStatus.modified ||
                      dag.status === SyncStatus.untracked) && (
                      <Checkbox
                        checked={selectedDags.has(dagId)}
                        onCheckedChange={() => handleToggleDag(dagId)}
                        aria-label={`Select ${dagId}`}
                      />
                    )}
                  </TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center gap-1.5">
                      <a
                        href={`/dags/${encodeURIComponent(dagId)}`}
                        className="font-mono hover:underline"
                      >
                        {formatSyncDisplayName(dagId, kind)}
                      </a>
                      {kind === 'memory' && (
                        <span className="text-[10px] px-1 py-0 rounded bg-purple-500/10 text-purple-600 dark:text-purple-400">
                          memory
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <StatusDot status={dag.status} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {dag.lastSyncedAt ? dayjs(dag.lastSyncedAt).fromNow() : '-'}
                  </TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center gap-0.5">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0"
                        onClick={() => handleViewDiff(dagId)}
                        title="View diff"
                      >
                        <FileCode className="h-3 w-3" />
                      </Button>
                      {(dag.status === SyncStatus.modified ||
                        dag.status === SyncStatus.untracked ||
                        dag.status === SyncStatus.conflict) &&
                        config?.pushEnabled &&
                        canWrite && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 w-6 p-0"
                            onClick={() =>
                              setPublishModal({ open: true, dagId })
                            }
                            title="Publish"
                          >
                            <Upload className="h-3 w-3" />
                          </Button>
                        )}
                      {(dag.status === SyncStatus.modified ||
                        dag.status === SyncStatus.conflict) &&
                        canWrite && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 w-6 p-0 text-muted-foreground hover:text-rose-600"
                            onClick={() =>
                              setRevertModal({ open: true, dagId })
                            }
                            title="Discard"
                          >
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Configuration Modal */}
      <Dialog open={showConfig} onOpenChange={setShowConfig}>
        <DialogContent className="sm:max-w-sm">
          <div className="space-y-3 text-sm">
            <div>
              <span className="text-muted-foreground text-xs">Repository</span>
              <div className="font-mono text-xs mt-0.5 select-all overflow-x-auto whitespace-nowrap scrollbar-thin">
                {config?.repository || '-'}
              </div>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Branch</span>
              <span className="font-mono text-xs">{config?.branch || '-'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Push</span>
              <span className="text-xs">
                {config?.pushEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Auto-sync</span>
              <span className="text-xs">
                {config?.autoSync?.enabled
                  ? `Every ${config.autoSync.interval || 300}s`
                  : 'Off'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Last Sync</span>
              <span className="text-xs">
                {status?.lastSyncAt
                  ? dayjs(status.lastSyncAt).format('MMM D, HH:mm')
                  : 'Never'}
              </span>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="w-full mt-2"
              onClick={handleTestConnection}
            >
              Test Connection
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Publish Modal */}
      <Dialog
        open={publishModal.open}
        onOpenChange={(open) => setPublishModal({ open })}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base">
              {publishModal.dagId
                ? `Publish ${publishModal.dagId}`
                : `Publish ${selectedDags.size} Selected`}
            </DialogTitle>
            <DialogDescription className="text-xs">
              Enter a commit message for this change.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="commit-message" className="text-xs">
                Commit Message
              </Label>
              <Input
                id="commit-message"
                className="h-8 text-sm"
                placeholder={
                  publishModal.dagId
                    ? `Update ${publishModal.dagId}`
                    : 'Batch update'
                }
                value={commitMessage}
                onChange={(e) => setCommitMessage(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !isPublishing) {
                    e.preventDefault();
                    handlePublish(publishModal.dagId);
                  }
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPublishModal({ open: false })}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => handlePublish(publishModal.dagId)}
              disabled={isPublishing}
            >
              {isPublishing ? (
                <>
                  <RefreshCw className="h-3.5 w-3.5 mr-1 animate-spin" />
                  Publishing...
                </>
              ) : (
                <>
                  <Upload className="h-3.5 w-3.5 mr-1" />
                  Publish
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Diff Modal */}
      <DiffModal
        open={diffModal.open}
        onOpenChange={(open) => setDiffModal({ open })}
        dagId={diffModal.dagId || ''}
        status={diffData?.status}
        localContent={diffData?.localContent}
        remoteContent={diffData?.remoteContent}
        remoteCommit={diffData?.remoteCommit}
        remoteAuthor={diffData?.remoteAuthor}
        canPublish={
          canWrite &&
          config?.pushEnabled &&
          (diffData?.status === SyncStatus.modified ||
            diffData?.status === SyncStatus.untracked ||
            diffData?.status === SyncStatus.conflict)
        }
        canRevert={
          canWrite &&
          (diffData?.status === SyncStatus.modified ||
            diffData?.status === SyncStatus.conflict)
        }
        onPublish={() => {
          setPublishModal({ open: true, dagId: diffModal.dagId });
        }}
        onRevert={() => {
          if (diffModal.dagId) {
            setRevertModal({ open: true, dagId: diffModal.dagId });
          }
        }}
      />

      {/* Revert Confirmation Modal */}
      <ConfirmModal
        title="Discard Changes"
        buttonText="Discard"
        visible={revertModal.open}
        dismissModal={() => setRevertModal({ open: false })}
        onSubmit={() => {
          if (revertModal.dagId) {
            handleDiscard(revertModal.dagId);
          }
          setRevertModal({ open: false });
          setDiffModal({ open: false });
        }}
      >
        <p className="text-sm text-muted-foreground">
          Discard local changes to{' '}
          <span className="font-mono font-medium text-foreground">
            {revertModal.dagId}
          </span>
          ? This cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}
