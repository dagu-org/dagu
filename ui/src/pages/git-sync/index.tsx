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
import { useQuery, useClient } from '@/hooks/api';
import { cn } from '@/lib/utils';
import dayjs from '@/lib/dayjs';
import {
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronRight,
  Download,
  GitBranch,
  RefreshCw,
  Settings,
  Upload,
  X,
  Trash2,
} from 'lucide-react';
import { useContext, useEffect, useState } from 'react';
import { SyncStatusBadge } from '@/features/git-sync/components/SyncStatusBadge';
import { Link } from 'react-router-dom';

type SyncStatusResponse = components['schemas']['SyncStatusResponse'];
type SyncConfigResponse = components['schemas']['SyncConfigResponse'];

const summaryConfig: Record<SyncSummary, { label: string; className: string; icon: React.ReactNode }> = {
  [SyncSummary.synced]: {
    label: 'All Synced',
    className: 'text-green-600 dark:text-green-400',
    icon: <Check className="h-4 w-4" />,
  },
  [SyncSummary.pending]: {
    label: 'Changes Pending',
    className: 'text-yellow-600 dark:text-yellow-400',
    icon: <Upload className="h-4 w-4" />,
  },
  [SyncSummary.conflict]: {
    label: 'Conflicts Detected',
    className: 'text-red-600 dark:text-red-400',
    icon: <AlertTriangle className="h-4 w-4" />,
  },
  [SyncSummary.error]: {
    label: 'Sync Error',
    className: 'text-red-600 dark:text-red-400',
    icon: <X className="h-4 w-4" />,
  },
};

export default function GitSyncPage() {
  const appBarContext = useContext(AppBarContext);
  const client = useClient();

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
    <div className="flex flex-col gap-4 max-w-5xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">Git Sync</h1>
          <p className="text-sm text-muted-foreground">
            Synchronize DAG definitions with remote Git repository
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handlePull}
            disabled={isPulling}
          >
            <Download className={cn('h-4 w-4 mr-1', isPulling && 'animate-spin')} />
            Pull
          </Button>
          <Button
            size="sm"
            onClick={() => setPublishModal({ open: true })}
            disabled={!hasModifiedDags || !config?.pushEnabled}
            title={!config?.pushEnabled ? 'Push is disabled in read-only mode' : undefined}
          >
            <Upload className="h-4 w-4 mr-1" />
            Publish All
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => mutateStatus()}
            title="Refresh status"
          >
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Messages */}
      {actionError && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md flex items-center gap-2">
          <AlertTriangle className="h-4 w-4" />
          {actionError}
        </div>
      )}
      {actionSuccess && (
        <div className="p-3 text-sm text-green-600 dark:text-green-400 bg-green-500/10 rounded-md flex items-center gap-2">
          <Check className="h-4 w-4" />
          {actionSuccess}
        </div>
      )}

      {/* Status Card */}
      <div className="card-obsidian p-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <div className="text-muted-foreground text-xs mb-1">Repository</div>
            <div className="font-medium truncate" title={status?.repository}>
              {status?.repository || '-'}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground text-xs mb-1">Branch</div>
            <div className="font-medium">{status?.branch || '-'}</div>
          </div>
          <div>
            <div className="text-muted-foreground text-xs mb-1">Last Sync</div>
            <div className="font-medium">
              {status?.lastSyncAt
                ? dayjs(status.lastSyncAt).format('MMM D, HH:mm')
                : 'Never'}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground text-xs mb-1">Status</div>
            <div className={cn('font-medium flex items-center gap-1', status?.summary && summaryConfig[status.summary]?.className)}>
              {status?.summary && summaryConfig[status.summary]?.icon}
              {status?.summary ? summaryConfig[status.summary]?.label : '-'}
            </div>
          </div>
        </div>

        {/* Status Counts */}
        {status?.counts && (
          <div className="flex items-center gap-4 mt-4 pt-4 border-t border-border/40 text-sm">
            <button
              onClick={() => setFilter('all')}
              className={cn('hover:underline', filter === 'all' && 'font-semibold')}
            >
              All ({Object.keys(status.dags || {}).length})
            </button>
            <button
              onClick={() => setFilter('modified')}
              className={cn('text-yellow-600 dark:text-yellow-400 hover:underline', filter === 'modified' && 'font-semibold')}
            >
              Modified ({status.counts.modified || 0})
            </button>
            <button
              onClick={() => setFilter('untracked')}
              className={cn('text-blue-600 dark:text-blue-400 hover:underline', filter === 'untracked' && 'font-semibold')}
            >
              Untracked ({status.counts.untracked || 0})
            </button>
            <button
              onClick={() => setFilter('conflict')}
              className={cn('text-red-600 dark:text-red-400 hover:underline', filter === 'conflict' && 'font-semibold')}
            >
              Conflict ({status.counts.conflict || 0})
            </button>
          </div>
        )}
      </div>

      {/* DAGs Table */}
      <div className="card-obsidian">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>DAG</TableHead>
              <TableHead className="w-[100px]">Status</TableHead>
              <TableHead className="w-[140px]">Last Synced</TableHead>
              <TableHead className="w-[100px]">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredDags.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                  {filter === 'all' ? 'No DAGs found' : `No ${filter} DAGs`}
                </TableCell>
              </TableRow>
            ) : (
              filteredDags.map(([dagId, dag]) => (
                <TableRow key={dagId}>
                  <TableCell>
                    <Link
                      to={`/dags/${encodeURIComponent(dagId)}`}
                      className="font-medium hover:underline"
                    >
                      {dagId}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <SyncStatusBadge status={dag.status} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {dag.lastSyncedAt
                      ? dayjs(dag.lastSyncedAt).format('MMM D, HH:mm')
                      : '-'}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      {(dag.status === SyncStatus.modified || dag.status === SyncStatus.untracked || dag.status === SyncStatus.conflict) && config?.pushEnabled && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2"
                          onClick={() => setPublishModal({ open: true, dagId })}
                          title="Publish to remote"
                        >
                          <Upload className="h-3.5 w-3.5" />
                        </Button>
                      )}
                      {(dag.status === SyncStatus.modified || dag.status === SyncStatus.conflict) && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 text-destructive hover:text-destructive"
                          onClick={() => handleDiscard(dagId)}
                          title="Discard local changes"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
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

      {/* Configuration Section */}
      <div className="card-obsidian">
        <button
          onClick={() => setShowConfig(!showConfig)}
          className="w-full flex items-center justify-between p-4 hover:bg-white/5 transition-colors"
        >
          <div className="flex items-center gap-2">
            <Settings className="h-4 w-4 text-muted-foreground" />
            <span className="font-medium">Configuration</span>
          </div>
          {showConfig ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
        </button>

        {showConfig && config && (
          <div className="p-4 pt-0 border-t border-border/40 space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <div className="text-muted-foreground text-xs mb-1">Push Enabled</div>
                <div className="font-medium">
                  {config.pushEnabled ? 'Yes' : 'No (Read-only mode)'}
                </div>
              </div>
              <div>
                <div className="text-muted-foreground text-xs mb-1">Auto-Sync</div>
                <div className="font-medium">
                  {config.autoSync?.enabled
                    ? `Every ${config.autoSync.interval || 300}s`
                    : 'Disabled'}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" onClick={handleTestConnection}>
                Test Connection
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* Publish Modal */}
      <Dialog open={publishModal.open} onOpenChange={(open) => setPublishModal({ open })}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {publishModal.dagId ? `Publish ${publishModal.dagId}` : 'Publish All Modified DAGs'}
            </DialogTitle>
            <DialogDescription>
              Enter a commit message for this change.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="commit-message">Commit Message</Label>
              <Input
                id="commit-message"
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
            <Button variant="outline" onClick={() => setPublishModal({ open: false })}>
              Cancel
            </Button>
            <Button onClick={() => handlePublish(publishModal.dagId)} disabled={isPublishing}>
              {isPublishing ? (
                <>
                  <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
                  Publishing...
                </>
              ) : (
                <>
                  <Upload className="h-4 w-4 mr-1" />
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
