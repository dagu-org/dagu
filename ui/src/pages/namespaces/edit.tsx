import { components, UserRole } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { AppBarContext } from '@/contexts/AppBarContext';
import { TOKEN_KEY, useIsAdmin } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import DAGEditor from '@/features/dags/components/dag-editor/DAGEditor';
import { useClient } from '@/hooks/api';
import { AlertCircle, ArrowLeft, Save, Trash2 } from 'lucide-react';
import { useCallback, useContext, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

type Namespace = components['schemas']['Namespace'];
type User = components['schemas']['User'];

const ALL_ROLES: UserRole[] = [UserRole.admin, UserRole.manager, UserRole.operator, UserRole.viewer];

const PERMISSION_MATRIX: Record<string, Record<UserRole, boolean>> = {
  'View DAGs & runs': { [UserRole.admin]: true, [UserRole.manager]: true, [UserRole.operator]: true, [UserRole.viewer]: true },
  'Run / stop DAGs': { [UserRole.admin]: true, [UserRole.manager]: true, [UserRole.operator]: true, [UserRole.viewer]: false },
  'Create / edit / delete DAGs': { [UserRole.admin]: true, [UserRole.manager]: true, [UserRole.operator]: false, [UserRole.viewer]: false },
  'Manage namespace settings': { [UserRole.admin]: true, [UserRole.manager]: false, [UserRole.operator]: false, [UserRole.viewer]: false },
};

export default function NamespaceEditPage() {
  const client = useClient();
  const navigate = useNavigate();
  const { name } = useParams<{ name: string }>();
  const appBarContext = useContext(AppBarContext);

  const [namespace, setNamespace] = useState<Namespace | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Form fields
  const [description, setDescription] = useState('');
  const [defaultQueue, setDefaultQueue] = useState('');
  const [defaultWorkingDir, setDefaultWorkingDir] = useState('');
  const [baseConfig, setBaseConfig] = useState('');
  const [baseConfigError, setBaseConfigError] = useState<string | null>(null);

  // Delete state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [dagCount, setDagCount] = useState<number | undefined>(undefined);

  // Git sync fields
  const [gitRemoteURL, setGitRemoteURL] = useState('');
  const [gitBranch, setGitBranch] = useState('');
  const [gitSSHKeyRef, setGitSSHKeyRef] = useState('');
  const [gitPath, setGitPath] = useState('');
  const [gitAutoSyncInterval, setGitAutoSyncInterval] = useState('');

  // Role assignment state
  const isAdmin = useIsAdmin();
  const config = useConfig();
  const [users, setUsers] = useState<User[]>([]);
  const [roleChanges, setRoleChanges] = useState<Record<string, UserRole | 'none'>>({});
  const [isSavingRoles, setIsSavingRoles] = useState(false);
  const [roleError, setRoleError] = useState<string | null>(null);
  const [roleSuccess, setRoleSuccess] = useState<string | null>(null);

  useEffect(() => {
    appBarContext.setTitle(`Edit Namespace: ${name || ''}`);
  }, [appBarContext, name]);

  const fetchNamespace = useCallback(async () => {
    if (!name) return;
    try {
      const { data, error: apiError } = await client.GET(
        '/namespaces/{namespaceName}',
        {
          params: { path: { namespaceName: name } },
        }
      );

      if (apiError) {
        throw new Error('Failed to fetch namespace');
      }

      const ns = data?.namespace;
      if (ns) {
        setNamespace(ns);
        setDescription(ns.description || '');
        setDefaultQueue(ns.defaults?.queue || '');
        setDefaultWorkingDir(ns.defaults?.workingDir || '');
        setGitRemoteURL(ns.gitSync?.remoteURL || '');
        setGitBranch(ns.gitSync?.branch || '');
        setGitSSHKeyRef(ns.gitSync?.sshKeyRef || '');
        setGitPath(ns.gitSync?.path || '');
        setGitAutoSyncInterval(ns.gitSync?.autoSyncInterval || '');
        setBaseConfig(ns.baseConfig || '');
      }
    } catch (err) {
      setFetchError(
        err instanceof Error ? err.message : 'Failed to load namespace'
      );
    } finally {
      setIsLoading(false);
    }
  }, [client, name]);

  useEffect(() => {
    fetchNamespace();
  }, [fetchNamespace]);

  // Fetch DAG count to determine if delete should be enabled
  useEffect(() => {
    if (!name) return;
    (async () => {
      try {
        const { data } = await client.GET(
          '/namespaces/{namespaceName}/dags',
          {
            params: {
              path: { namespaceName: name },
              query: { perPage: 1 },
            },
          }
        );
        setDagCount(data?.pagination?.totalRecords ?? 0);
      } catch {
        setDagCount(undefined);
      }
    })();
  }, [client, name]);

  // Fetch users for role assignment
  const fetchUsers = useCallback(async () => {
    if (!isAdmin) return;
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const response = await fetch(`${config.apiURL}/users`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!response.ok) return;
      const data = await response.json();
      setUsers(data.users || []);
    } catch {
      // Silently fail - role section just won't show
    }
  }, [isAdmin, config.apiURL]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const getUserNsRole = (user: User): UserRole | 'none' => {
    const changed = roleChanges[user.id];
    if (changed !== undefined) return changed;
    if (!name || !user.namespaceRoles) return 'none';
    const role = user.namespaceRoles[name];
    return role ? (role as UserRole) : 'none';
  };

  const handleRoleChange = (userId: string, role: UserRole | 'none') => {
    setRoleChanges((prev) => ({ ...prev, [userId]: role }));
    setRoleSuccess(null);
  };

  const handleSaveRoles = async () => {
    if (!name) return;
    setIsSavingRoles(true);
    setRoleError(null);
    setRoleSuccess(null);

    try {
      const token = localStorage.getItem(TOKEN_KEY);

      for (const [userId, newRole] of Object.entries(roleChanges)) {
        const user = users.find((u) => u.id === userId);
        if (!user) continue;

        // Build the full namespace roles map
        const nsRoles: Record<string, string> = { ...(user.namespaceRoles || {}) };
        if (newRole === 'none') {
          delete nsRoles[name];
        } else {
          nsRoles[name] = newRole;
        }

        const response = await fetch(`${config.apiURL}/users/${userId}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ namespaceRoles: nsRoles }),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(
            (data as { message?: string }).message ||
              `Failed to update roles for ${user.username}`
          );
        }
      }

      setRoleChanges({});
      setRoleSuccess('Role assignments saved');
      fetchUsers();
    } catch (err) {
      setRoleError(
        err instanceof Error ? err.message : 'Failed to save role assignments'
      );
    } finally {
      setIsSavingRoles(false);
    }
  };

  const handleDelete = async () => {
    if (!name) return;
    setIsDeleting(true);
    setError(null);

    try {
      const { error: apiError } = await client.DELETE(
        '/namespaces/{namespaceName}',
        {
          params: { path: { namespaceName: name } },
        }
      );

      if (apiError) {
        const msg =
          (apiError as { message?: string }).message ||
          'Failed to delete namespace';
        throw new Error(msg);
      }

      navigate('/namespaces');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to delete namespace'
      );
      setShowDeleteDialog(false);
    } finally {
      setIsDeleting(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name) return;
    setError(null);
    setBaseConfigError(null);
    setIsSubmitting(true);

    try {
      const body: components['schemas']['UpdateNamespaceRequest'] = {};

      body.description = description;

      if (defaultQueue || defaultWorkingDir) {
        body.defaults = {};
        if (defaultQueue) {
          body.defaults.queue = defaultQueue;
        }
        if (defaultWorkingDir) {
          body.defaults.workingDir = defaultWorkingDir;
        }
      }

      if (gitRemoteURL || gitBranch || gitSSHKeyRef || gitPath || gitAutoSyncInterval) {
        body.gitSync = {};
        if (gitRemoteURL) body.gitSync.remoteURL = gitRemoteURL;
        if (gitBranch) body.gitSync.branch = gitBranch;
        if (gitSSHKeyRef) body.gitSync.sshKeyRef = gitSSHKeyRef;
        if (gitPath) body.gitSync.path = gitPath;
        if (gitAutoSyncInterval) body.gitSync.autoSyncInterval = gitAutoSyncInterval;
      }

      // Include base config YAML if it has content
      if (baseConfig.trim()) {
        body.baseConfig = baseConfig;
      }

      const { error: apiError } = await client.PUT(
        '/namespaces/{namespaceName}',
        {
          params: { path: { namespaceName: name } },
          body,
        }
      );

      if (apiError) {
        const msg =
          (apiError as { message?: string }).message ||
          'Failed to update namespace';
        // Show base config validation errors inline
        if (msg.includes('base config YAML')) {
          setBaseConfigError(msg);
          return;
        }
        throw new Error(msg);
      }

      navigate('/namespaces');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to update namespace'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4 max-w-4xl">
        <p className="text-sm text-muted-foreground py-8 text-center">
          Loading namespace...
        </p>
      </div>
    );
  }

  if (fetchError || !namespace) {
    return (
      <div className="flex flex-col gap-4 max-w-4xl">
        <Button
          variant="ghost"
          size="sm"
          className="mb-2 -ml-2 text-muted-foreground w-fit"
          onClick={() => navigate('/namespaces')}
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Namespaces
        </Button>
        <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          <AlertCircle className="h-4 w-4 flex-shrink-0" />
          <span>{fetchError || 'Namespace not found'}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 max-w-4xl">
      <div>
        <Button
          variant="ghost"
          size="sm"
          className="mb-2 -ml-2 text-muted-foreground"
          onClick={() => navigate('/namespaces')}
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Namespaces
        </Button>
        <h1 className="text-lg font-semibold">Edit Namespace: {namespace.name}</h1>
        <p className="text-sm text-muted-foreground">
          ID: {namespace.id}
        </p>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          <AlertCircle className="h-4 w-4 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* General Settings */}
        <div className="card-obsidian p-6 space-y-5">
          <h2 className="text-sm font-medium">General Settings</h2>

          <div className="space-y-1.5">
            <Label htmlFor="description" className="text-sm">
              Description
            </Label>
            <Input
              id="description"
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description for this namespace"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="defaultQueue" className="text-sm">
              Default Queue
            </Label>
            <Input
              id="defaultQueue"
              type="text"
              value={defaultQueue}
              onChange={(e) => setDefaultQueue(e.target.value)}
              placeholder="Optional default queue for DAGs in this namespace"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="defaultWorkingDir" className="text-sm">
              Default Working Directory
            </Label>
            <Input
              id="defaultWorkingDir"
              type="text"
              value={defaultWorkingDir}
              onChange={(e) => setDefaultWorkingDir(e.target.value)}
              placeholder="Optional default working directory"
              autoComplete="off"
              className="h-9"
            />
          </div>
        </div>

        {/* Git Sync Settings */}
        <div className="card-obsidian p-6 space-y-5">
          <div>
            <h2 className="text-sm font-medium">Git Sync Settings</h2>
            <p className="text-xs text-muted-foreground mt-1">
              Configure git synchronization for DAG definitions in this namespace
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gitRemoteURL" className="text-sm">
              Remote URL
            </Label>
            <Input
              id="gitRemoteURL"
              type="text"
              value={gitRemoteURL}
              onChange={(e) => setGitRemoteURL(e.target.value)}
              placeholder="e.g. git@github.com:org/repo.git"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gitBranch" className="text-sm">
              Branch
            </Label>
            <Input
              id="gitBranch"
              type="text"
              value={gitBranch}
              onChange={(e) => setGitBranch(e.target.value)}
              placeholder="e.g. main"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gitSSHKeyRef" className="text-sm">
              SSH Key Reference
            </Label>
            <Input
              id="gitSSHKeyRef"
              type="text"
              value={gitSSHKeyRef}
              onChange={(e) => setGitSSHKeyRef(e.target.value)}
              placeholder="Path or reference to SSH key for authentication"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gitPath" className="text-sm">
              Path
            </Label>
            <Input
              id="gitPath"
              type="text"
              value={gitPath}
              onChange={(e) => setGitPath(e.target.value)}
              placeholder="Subdirectory within the repo for this namespace"
              autoComplete="off"
              className="h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gitAutoSyncInterval" className="text-sm">
              Sync Interval
            </Label>
            <Input
              id="gitAutoSyncInterval"
              type="text"
              value={gitAutoSyncInterval}
              onChange={(e) => setGitAutoSyncInterval(e.target.value)}
              placeholder="e.g. 5m, 1h"
              autoComplete="off"
              className="h-9"
            />
            <p className="text-xs text-muted-foreground">
              Interval for automatic syncing (e.g. 5m, 1h, 30s)
            </p>
          </div>
        </div>

        {/* Base Configuration */}
        <div className="card-obsidian p-6 space-y-3">
          <div>
            <h2 className="text-sm font-medium">Base Configuration</h2>
            <p className="text-xs text-muted-foreground mt-1">
              YAML configuration applied as defaults to all DAGs in this
              namespace (e.g. env, logDir, handlerOn, histRetentionDays)
            </p>
          </div>

          {baseConfigError && (
            <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{baseConfigError}</span>
            </div>
          )}

          <div className="border border-border rounded-lg overflow-hidden h-[400px]">
            <DAGEditor
              value={baseConfig}
              onChange={(val) => {
                setBaseConfig(val || '');
                setBaseConfigError(null);
              }}
              readOnly={false}
              lineNumbers={true}
            />
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => navigate('/namespaces')}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            <Save className="h-4 w-4" />
            {isSubmitting ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </form>

      {/* Role Assignment - admin only */}
      {isAdmin && users.length > 0 && (
        <div className="card-obsidian p-6 space-y-4">
          <div>
            <h2 className="text-sm font-medium">Role Assignment</h2>
            <p className="text-xs text-muted-foreground mt-1">
              Assign per-namespace roles to users. A user&apos;s effective role is
              the higher of their global role and namespace-specific role.
            </p>
          </div>

          {roleError && (
            <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{roleError}</span>
            </div>
          )}

          {roleSuccess && (
            <div className="p-3 text-sm text-green-700 dark:text-green-400 bg-green-500/10 rounded-md">
              {roleSuccess}
            </div>
          )}

          <Table className="text-xs">
            <TableHeader>
              <TableRow>
                <TableHead className="w-[200px]">User</TableHead>
                <TableHead className="w-[120px]">Global Role</TableHead>
                <TableHead className="w-[160px]">Namespace Role</TableHead>
                <TableHead className="w-[120px]">Effective</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => {
                const nsRole = getUserNsRole(user);
                const globalRole = user.role;
                const effectiveRole =
                  nsRole !== 'none'
                    ? ALL_ROLES[
                        Math.min(
                          ALL_ROLES.indexOf(globalRole),
                          ALL_ROLES.indexOf(nsRole)
                        )
                      ]
                    : globalRole;
                return (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium">
                      {user.username}
                    </TableCell>
                    <TableCell>
                      <span className="px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                        {globalRole}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Select
                        value={nsRole}
                        onValueChange={(val) =>
                          handleRoleChange(
                            user.id,
                            val as UserRole | 'none'
                          )
                        }
                      >
                        <SelectTrigger className="h-7 text-xs w-[130px]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">
                            <span className="text-muted-foreground">
                              None (global only)
                            </span>
                          </SelectItem>
                          {ALL_ROLES.map((r) => (
                            <SelectItem key={r} value={r}>
                              <span className="capitalize">{r}</span>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <span className="px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                        {effectiveRole}
                      </span>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>

          {Object.keys(roleChanges).length > 0 && (
            <div className="flex justify-end">
              <Button
                size="sm"
                onClick={handleSaveRoles}
                disabled={isSavingRoles}
              >
                <Save className="h-4 w-4" />
                {isSavingRoles ? 'Saving...' : 'Save Roles'}
              </Button>
            </div>
          )}

          {/* Permission Matrix */}
          <div className="mt-4">
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              Permission Matrix
            </h3>
            <Table className="text-xs">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[220px]">Permission</TableHead>
                  {ALL_ROLES.map((r) => (
                    <TableHead key={r} className="w-[80px] text-center capitalize">
                      {r}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {Object.entries(PERMISSION_MATRIX).map(
                  ([permission, roles]) => (
                    <TableRow key={permission}>
                      <TableCell>{permission}</TableCell>
                      {ALL_ROLES.map((r) => (
                        <TableCell key={r} className="text-center">
                          {roles[r] ? (
                            <span className="text-green-600 dark:text-green-400">
                              &#10003;
                            </span>
                          ) : (
                            <span className="text-muted-foreground">
                              &mdash;
                            </span>
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                  )
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      {/* Danger Zone - only for non-default namespaces */}
      {namespace.name !== 'default' && (
        <div className="card-obsidian p-6 space-y-4 border-destructive/30">
          <div>
            <h2 className="text-sm font-medium text-destructive">
              Danger Zone
            </h2>
            <p className="text-xs text-muted-foreground mt-1">
              Permanently delete this namespace. This action cannot be undone.
            </p>
          </div>
          <Button
            variant="destructive"
            size="sm"
            disabled={dagCount !== undefined && dagCount > 0}
            title={
              dagCount !== undefined && dagCount > 0
                ? 'Cannot delete namespace with DAGs'
                : 'Delete this namespace'
            }
            onClick={() => setShowDeleteDialog(true)}
          >
            <Trash2 className="h-4 w-4" />
            {dagCount !== undefined && dagCount > 0
              ? `Delete Namespace (${dagCount} DAGs exist)`
              : 'Delete Namespace'}
          </Button>
        </div>
      )}

      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Namespace</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete the namespace{' '}
              <strong>{namespace.name}</strong>? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setShowDeleteDialog(false)}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
