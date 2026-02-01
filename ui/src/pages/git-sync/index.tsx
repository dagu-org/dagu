import { components, SyncStatus, SyncSummary } from '@/api/v2/schema';
import { Button } from '@/components/ui/button';
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
import { useContext, useEffect, useState } from 'react';
import { DiffModal } from './DiffModal';

type SyncStatusResponse = components['schemas']['SyncStatusResponse'];
type SyncConfigResponse = components['schemas']['SyncConfigResponse'];
type SyncDAGDiffResponse = components['schemas']['SyncDAGDiffResponse'];

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
  const [filter, setFilter] = useState<
    'all' | 'modified' | 'untracked' | 'conflict'
  >('all');
  const [diffModal, setDiffModal] = useState<{ open: boolean; dagId?: string }>(
    { open: false }
  );
  const [diffData, setDiffData] = useState<SyncDAGDiffResponse | null>(null);
  const [revertModal, setRevertModal] = useState<{
    open: boolean;
    dagId?: string;
  }>({ open: false });

  useEffect(() => {
    appBarContext.setTitle('Git Sync');
  }, [appBarContext.setTitle]);

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
            body: { message: commitMessage || 'Batch update' },
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

  // Filter DAGs by status
  const filteredDags = status?.dags
    ? Object.entries(status.dags).filter(([, dag]) => {
        if (filter === 'all') return true;
        return dag.status === filter;
      })
    : [];

  const hasModifiedDags =
    status?.counts &&
    ((status.counts.modified || 0) > 0 || (status.counts.untracked || 0) > 0);

  const getFilterCount = (f: string): number => {
    if (!status?.counts) return 0;
    if (f === 'all') return Object.keys(status.dags || {}).length;
    return status.counts[f as keyof typeof status.counts] || 0;
  };

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
    <div className="flex flex-col gap-4 max-w-7xl mx-auto pb-8">
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
            disabled={!hasModifiedDags || !config?.pushEnabled || !canWrite}
            title={
              !canWrite
                ? 'Write permission required'
                : !config?.pushEnabled
                  ? 'Push disabled in read-only mode'
                  : 'Publish all changes'
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

      {/* Filter Tabs */}
      <div
        className="flex items-center gap-1 text-xs border-b border-border/40"
        role="tablist"
        aria-label="Filter DAGs by status"
      >
        {(['all', 'modified', 'untracked', 'conflict'] as const).map((f, index) => (
          <button
            key={f}
            role="tab"
            aria-selected={filter === f}
            tabIndex={filter === f ? 0 : -1}
            onClick={() => setFilter(f)}
            onKeyDown={(e) => {
              const filters = ['all', 'modified', 'untracked', 'conflict'] as const;
              if (e.key === 'ArrowRight') {
                e.preventDefault();
                const nextFilter = filters[(index + 1) % filters.length];
                if (nextFilter) setFilter(nextFilter);
              } else if (e.key === 'ArrowLeft') {
                e.preventDefault();
                const prevFilter = filters[(index - 1 + filters.length) % filters.length];
                if (prevFilter) setFilter(prevFilter);
              }
            }}
            className={cn(
              'px-3 py-1.5 border-b-2 -mb-px transition-colors focus:outline-none',
              filter === f
                ? 'border-foreground text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            {f.charAt(0).toUpperCase() + f.slice(1)} ({getFilterCount(f)})
          </button>
        ))}
      </div>

      {/* DAGs Table */}
      <div className="bg-card border border-border rounded-md overflow-hidden shadow-sm">
        <Table className="text-sm">
          <TableHeader>
            <TableRow className="text-xs">
              <TableHead className="py-2">DAG</TableHead>
              <TableHead className="py-2 w-24">Status</TableHead>
              <TableHead className="py-2 w-28">Synced</TableHead>
              <TableHead className="py-2 w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredDags.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className="text-center text-muted-foreground py-8 text-xs"
                >
                  {filter === 'all' ? 'No DAGs found' : `No ${filter} DAGs`}
                </TableCell>
              </TableRow>
            ) : (
              filteredDags.map(([dagId, dag]) => (
                <TableRow
                  key={dagId}
                  className="h-9 cursor-pointer hover:bg-muted/50"
                  onClick={() => handleViewDiff(dagId)}
                >
                  <TableCell
                    className="py-1"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <a
                      href={`/dags/${encodeURIComponent(dagId)}`}
                      className="font-mono text-xs hover:underline"
                    >
                      {dagId}
                    </a>
                  </TableCell>
                  <TableCell className="py-1">
                    <StatusDot status={dag.status} />
                  </TableCell>
                  <TableCell className="py-1 text-xs text-muted-foreground">
                    {dag.lastSyncedAt ? dayjs(dag.lastSyncedAt).fromNow() : '-'}
                  </TableCell>
                  <TableCell
                    className="py-1"
                    onClick={(e) => e.stopPropagation()}
                  >
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
                : 'Publish All'}
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
