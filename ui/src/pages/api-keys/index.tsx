import { useState, useEffect, useCallback, useContext } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { useIsAdmin, TOKEN_KEY } from '@/contexts/AuthContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { components } from '@/api/v2/schema';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { KeyRound, MoreHorizontal, Pencil, Trash2, Shield, Plus } from 'lucide-react';
import { APIKeyFormModal } from './APIKeyFormModal';
import ConfirmModal from '@/ui/ConfirmModal';
import dayjs from '@/lib/dayjs';

type APIKey = components['schemas']['APIKey'];

export default function APIKeysPage() {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingKey, setEditingKey] = useState<APIKey | null>(null);
  const [deletingKey, setDeletingKey] = useState<APIKey | null>(null);

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('API Keys');
  }, [appBarContext]);

  const fetchAPIKeys = useCallback(async () => {
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const response = await fetch(`${config.apiURL}/api-keys`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch API keys');
      }

      const data = await response.json();
      setApiKeys(data.apiKeys || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load API keys');
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL]);

  useEffect(() => {
    fetchAPIKeys();
  }, [fetchAPIKeys]);

  const handleDeleteKey = async () => {
    if (!deletingKey) return;

    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const response = await fetch(`${config.apiURL}/api-keys/${deletingKey.id}`, {
        method: 'DELETE',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to delete API key');
      }

      setError(null);
      setDeletingKey(null);
      fetchAPIKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete API key');
    }
  };

  const getRoleBadgeColor = (role: string) => {
    switch (role) {
      case 'admin':
        return 'bg-error/20 text-error';
      case 'manager':
        return 'bg-primary/100/20 text-primary';
      case 'operator':
        return 'bg-success/20 text-success';
      case 'viewer':
        return 'bg-muted-foreground/20 text-muted-foreground';
      default:
        return 'bg-muted-foreground/20 text-muted-foreground';
    }
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">You do not have permission to access this page.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">API Keys</h1>
          <p className="text-sm text-muted-foreground">
            Manage API keys for programmatic access
          </p>
        </div>
        <Button onClick={() => setShowCreateModal(true)} size="sm" className="h-8">
          <Plus className="h-4 w-4 mr-1.5" />
          Create API Key
        </Button>
      </div>

      {error && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          {error}
        </div>
      )}

      <div className="border rounded-lg">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[200px]">Name</TableHead>
              <TableHead className="w-[120px]">Role</TableHead>
              <TableHead className="w-[100px]">Key Prefix</TableHead>
              <TableHead className="w-[180px]">Created</TableHead>
              <TableHead className="w-[180px]">Last Used</TableHead>
              <TableHead className="w-[80px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                  Loading API keys...
                </TableCell>
              </TableRow>
            ) : apiKeys.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                  No API keys found. Create one to get started.
                </TableCell>
              </TableRow>
            ) : (
              apiKeys.map((key) => (
                <TableRow key={key.id}>
                  <TableCell className="font-medium">
                    <div className="flex flex-col">
                      <div className="flex items-center gap-2">
                        <KeyRound className="h-3.5 w-3.5 text-muted-foreground" />
                        {key.name}
                      </div>
                      {key.description && (
                        <span className="text-xs text-muted-foreground ml-5">
                          {key.description}
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <Shield className="h-3.5 w-3.5 text-muted-foreground" />
                      <span
                        className={`text-xs px-1.5 py-0.5 rounded-full capitalize ${getRoleBadgeColor(key.role)}`}
                      >
                        {key.role}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                      {key.keyPrefix}...
                    </code>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {dayjs(key.createdAt).format('MMM D, YYYY HH:mm')}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {key.lastUsedAt
                      ? dayjs(key.lastUsedAt).format('MMM D, YYYY HH:mm')
                      : 'Never'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditingKey(key)}>
                          <Pencil className="h-4 w-4 mr-2" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => setDeletingKey(key)}
                          className="text-destructive"
                        >
                          <Trash2 className="h-4 w-4 mr-2" />
                          Revoke
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Create API Key Modal */}
      <APIKeyFormModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSuccess={() => {
          setShowCreateModal(false);
          fetchAPIKeys();
        }}
      />

      {/* Edit API Key Modal */}
      <APIKeyFormModal
        open={!!editingKey}
        apiKey={editingKey || undefined}
        onClose={() => setEditingKey(null)}
        onSuccess={() => {
          setEditingKey(null);
          fetchAPIKeys();
        }}
      />

      {/* Delete Confirmation */}
      <ConfirmModal
        title="Revoke API Key"
        buttonText="Revoke"
        visible={!!deletingKey}
        dismissModal={() => setDeletingKey(null)}
        onSubmit={handleDeleteKey}
      >
        <p>
          Are you sure you want to revoke the API key &quot;{deletingKey?.name}&quot;?
          Any applications using this key will immediately lose access.
        </p>
      </ConfirmModal>
    </div>
  );
}
