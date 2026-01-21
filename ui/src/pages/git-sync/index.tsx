import { components, SyncStatus, SyncSummary } from '@/api/v2/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useCanWrite } from '@/contexts/AuthContext';
import { useQuery, useClient } from '@/hooks/api';
import { cn } from '@/lib/utils';
import dayjs from '@/lib/dayjs';
import {
  AlertTriangle,
  Check,
  Download,
  GitBranch,
  RefreshCw,
  Settings,
  Upload,
  Trash2,
} from 'lucide-react';
import { useContext, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

type SyncStatusResponse = components['schemas']['SyncStatusResponse'];
type SyncConfigResponse = components['schemas']['SyncConfigResponse'];

// Subtle, readable status colors
const summaryConfig: Record<SyncSummary, { label: string; badgeClass: string }> = {
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

// Status dot component - simple colored dots like GitHub
function StatusDot({ status }: { status: SyncStatus }) {
  const colors: Record<SyncStatus, string> = {
    [SyncStatus.synced]: 'bg-emerald-500',
    [SyncStatus.modified]: 'bg-amber-500',
    [SyncStatus.untracked]: 'bg-blue-500',
    [SyncStatus.conflict]: 'bg-rose-500',
  };
  const labels: Record<SyncStatus, string> = {
    [SyncStatus.synced]: 'synced',
    [SyncStatus.modified]: 'modified',
    [SyncStatus.untracked]: 'untracked',
    [SyncStatus.conflict]: 'conflict',
  };
  return (
    <div className="flex items-center gap-1.5">
      <span className={cn('h-2 w-2 rounded-full', colors[status])} />
      <span className="text-xs text-muted-foreground">{labels[status]}</span>
    </div>
  );
}

export default function GitSyncPage() {
  const appBarContext = useContext(AppBarContext);
  const client = useClient();
  const canWrite = useCanWrite();

  // State
  const [isPulling, setIsPulling] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [publishModal, setPublishModal] = useState<{ open: boolean; dagId?: string }>({ open: false });
  const [commitMessage, setCommitMessage] = useState('');
  const [filter, setFilter] = useState<'all' | 'modified' | 'untracked' | 'conflict'>('all');
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  useEffect(() => {
    appBarContext.setTitle('Git Sync');
  }, [appBarContext]);

  // Fetch sync status
  const {
    data: statusData,
    mutate: mutateStatus,
  } = useQuery(
    '/sync/status',
    {},
    { refreshInterval: 5000 }
  );

  // Fetch sync config
  const {
    data: configData,
  } = useQuery('/sync/config', {});

  const status = statusData as SyncStatusResponse | undefined;
  const config = configData as SyncConfigResponse | undefined;

  // Clear messages after 5 seconds
  useEffect(() => {
    if (actionError || actionSuccess) {
      const timer = setTimeout(() => {
        setActionError(null);
        setActionSuccess(null);
      }, 5000);
      return () => clearTimeout(timer);
    }
  }, [actionError, actionSuccess]);

  // Handlers
  const handlePull = async () => {
    setIsPulling(true);
    setActionError(null);
    try {
      const response = await client.POST('/sync/pull', {});
      if (response.error) {
        setActionError(response.error.message || 'Pull failed');
      } else {
        setActionSuccess(`Pull completed. ${response.data?.synced?.length || 0} DAGs synced.`);
        mutateStatus();
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Pull failed');
    } finally {
      setIsPulling(false);
    }
  };

  const handlePublish = async (dagId?: string, force?: boolean) => {
    setIsPublishing(true);
    setActionError(null);
    try {
      if (dagId) {
        // Publish single DAG
        const response = await client.POST('/dags/{name}/publish', {
          params: { path: { name: dagId } },
          body: { message: commitMessage || `Update ${dagId}`, force: force || false },
        });
        if (response.error) {
          setActionError(response.error.message || 'Publish failed');
        } else {
          setActionSuccess(`Published ${dagId} successfully.`);
          setPublishModal({ open: false });
          setCommitMessage('');
          mutateStatus();
        }
      } else {
        // Publish all modified DAGs
        const response = await client.POST('/sync/publish-all', {
          body: { message: commitMessage || 'Batch update' },
        });
        if (response.error) {
          setActionError(response.error.message || 'Publish failed');
        } else {
          setActionSuccess(`Published ${response.data?.synced?.length || 0} DAGs successfully.`);
          setPublishModal({ open: false });
          setCommitMessage('');
          mutateStatus();
        }
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Publish failed');
    } finally {
      setIsPublishing(false);
    }
  };

  const handleDiscard = async (dagId: string) => {
    if (!confirm(`Discard local changes to ${dagId}? This cannot be undone.`)) return;
    setActionError(null);
    try {
      const response = await client.POST('/dags/{name}/discard', {
        params: { path: { name: dagId } },
      });
      if (response.error) {
        setActionError(response.error.message || 'Discard failed');
      } else {
        setActionSuccess(`Discarded changes to ${dagId}.`);
        mutateStatus();
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Discard failed');
    }
  };

  const handleTestConnection = async () => {
    setActionError(null);
    try {
      const response = await client.POST('/sync/test-connection', {});
      if (response.error) {
        setActionError(response.error.message || 'Connection test failed');
      } else if (response.data?.success) {
        setActionSuccess('Connection successful!');
      } else {
        setActionError(response.data?.error || 'Connection test failed');
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Connection test failed');
    }
  };

  // Filter DAGs by status
  const filteredDags = status?.dags
    ? Object.entries(status.dags).filter(([, dag]) => {
        if (filter === 'all') return true;
        return dag.status === filter;
      })
    : [];

  const hasModifiedDags = status?.counts && (
    (status.counts.modified || 0) > 0 ||
    (status.counts.untracked || 0) > 0
  );

  const getFilterCount = (f: string) => {
    if (!status?.counts) return 0;
    if (f === 'all') return Object.keys(status.dags || {}).length;
    if (f === 'modified') return status.counts.modified || 0;
    if (f === 'untracked') return status.counts.untracked || 0;
    if (f === 'conflict') return status.counts.conflict || 0;
    return 0;
  };

  // Not configured state
  if (!status?.enabled) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] text-center">
        <GitBranch className="h-16 w-16 text-muted-foreground/30 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Git Sync Not Configured</h2>
        <p className="text-muted-foreground max-w-md">
          Git sync allows you to synchronize DAG definitions with a remote Git repository.
          Configure it in your server settings.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3 max-w-4xl mx-auto pb-8">
      {/* Compact Header */}
      <div className="flex items-center justify-between py-2">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold">Git Sync</h1>
          <span className="text-xs text-muted-foreground font-mono">
            {status?.repository}:{status?.branch}
          </span>
          {status?.summary && (
            <span className={cn('text-xs px-2 py-0.5 rounded', summaryConfig[status.summary]?.badgeClass)}>
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
            <Download className={cn('h-4 w-4', isPulling && 'animate-spin')} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={() => setPublishModal({ open: true })}
            disabled={!hasModifiedDags || !config?.pushEnabled || !canWrite}
            title={!canWrite ? 'Write permission required' : !config?.pushEnabled ? 'Push disabled in read-only mode' : 'Publish all changes'}
          >
            <Upload className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 w-8 p-0"
            onClick={() => mutateStatus()}
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

      {/* Messages */}
      {actionError && (
        <div className="p-2 text-xs text-rose-700 dark:text-rose-400 bg-rose-500/10 rounded flex items-center gap-2">
          <AlertTriangle className="h-3.5 w-3.5" />
          {actionError}
        </div>
      )}
      {actionSuccess && (
        <div className="p-2 text-xs text-emerald-700 dark:text-emerald-400 bg-emerald-500/10 rounded flex items-center gap-2">
          <Check className="h-3.5 w-3.5" />
          {actionSuccess}
        </div>
      )}

      {/* Filter Tabs */}
      <div className="flex items-center gap-1 text-xs border-b border-border/40">
        {(['all', 'modified', 'untracked', 'conflict'] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={cn(
              'px-3 py-1.5 border-b-2 -mb-px transition-colors',
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
      <div className="card-obsidian">
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
                <TableCell colSpan={4} className="text-center text-muted-foreground py-8 text-xs">
                  {filter === 'all' ? 'No DAGs found' : `No ${filter} DAGs`}
                </TableCell>
              </TableRow>
            ) : (
              filteredDags.map(([dagId, dag]) => (
                <TableRow key={dagId} className="h-9">
                  <TableCell className="py-1">
                    <Link
                      to={`/dags/${encodeURIComponent(dagId)}`}
                      className="font-mono text-xs hover:underline"
                    >
                      {dagId}
                    </Link>
                  </TableCell>
                  <TableCell className="py-1">
                    <StatusDot status={dag.status} />
                  </TableCell>
                  <TableCell className="py-1 text-xs text-muted-foreground">
                    {dag.lastSyncedAt ? dayjs(dag.lastSyncedAt).fromNow() : '-'}
                  </TableCell>
                  <TableCell className="py-1">
                    <div className="flex items-center gap-0.5">
                      {(dag.status === SyncStatus.modified || dag.status === SyncStatus.untracked || dag.status === SyncStatus.conflict) && config?.pushEnabled && canWrite && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 p-0"
                          onClick={() => setPublishModal({ open: true, dagId })}
                          title="Publish"
                        >
                          <Upload className="h-3 w-3" />
                        </Button>
                      )}
                      {(dag.status === SyncStatus.modified || dag.status === SyncStatus.conflict) && canWrite && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 p-0 text-muted-foreground hover:text-rose-600"
                          onClick={() => handleDiscard(dagId)}
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
              <span className="text-xs">{config?.pushEnabled ? 'Enabled' : 'Disabled'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Auto-sync</span>
              <span className="text-xs">
                {config?.autoSync?.enabled ? `Every ${config.autoSync.interval || 300}s` : 'Off'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Last Sync</span>
              <span className="text-xs">
                {status?.lastSyncAt ? dayjs(status.lastSyncAt).format('MMM D, HH:mm') : 'Never'}
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
      <Dialog open={publishModal.open} onOpenChange={(open) => setPublishModal({ open })}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base">
              {publishModal.dagId ? `Publish ${publishModal.dagId}` : 'Publish All'}
            </DialogTitle>
            <DialogDescription className="text-xs">
              Enter a commit message for this change.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="commit-message" className="text-xs">Commit Message</Label>
              <Input
                id="commit-message"
                className="h-8 text-sm"
                placeholder={publishModal.dagId ? `Update ${publishModal.dagId}` : 'Batch update'}
                value={commitMessage}
                onChange={(e) => setCommitMessage(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    handlePublish(publishModal.dagId);
                  }
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setPublishModal({ open: false })}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => handlePublish(publishModal.dagId)} disabled={isPublishing}>
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
    </div>
  );
}
