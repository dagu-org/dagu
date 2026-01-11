import { useState, useEffect, useCallback, useContext } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { useAuth, useIsAdmin, TOKEN_KEY } from '@/contexts/AuthContext';
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
import { UserPlus, MoreHorizontal, Pencil, Trash2, Key, Ban, UserCheck } from 'lucide-react';
import { UserFormModal } from './UserFormModal';
import { ResetPasswordModal } from './ResetPasswordModal';
import ConfirmModal from '@/ui/ConfirmModal';
import dayjs from '@/lib/dayjs';

type User = components['schemas']['User'];

/**
 * Render the Users management page with a table of accounts and controls for creating, editing, resetting passwords, and deleting users.
 *
 * This component sets the application bar title to "User Management", fetches the user list from the configured API using a stored token, and manages loading and error states. It highlights the current user, formats created/updated timestamps, and exposes per-user actions that open the appropriate modals (create, edit, reset password, delete). Deletion performs an API DELETE request and refreshes the list on success.
 *
 * @returns The Users page component as a JSX.Element
 */
export default function UsersPage() {
  const config = useConfig();
  const { user: currentUser } = useAuth();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const [users, setUsers] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [resetPasswordUser, setResetPasswordUser] = useState<User | null>(null);
  const [deletingUser, setDeletingUser] = useState<User | null>(null);

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('User Management');
  }, [appBarContext]);

  const fetchUsers = useCallback(async () => {
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');
      const response = await fetch(`${config.apiURL}/users?remoteNode=${remoteNode}`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch users');
      }

      const data = await response.json();
      setUsers(data.users || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users');
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL, appBarContext.selectedRemoteNode]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleDeleteUser = async () => {
    if (!deletingUser) return;

    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');
      const response = await fetch(`${config.apiURL}/users/${deletingUser.id}?remoteNode=${remoteNode}`, {
        method: 'DELETE',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to delete user');
      }

      setError(null); // Clear any previous error on success
      setDeletingUser(null);
      fetchUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete user');
    }
  };

  const handleToggleDisabled = async (user: User) => {
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');
      const response = await fetch(`${config.apiURL}/users/${user.id}?remoteNode=${remoteNode}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isDisabled: !user.isDisabled }),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to update user');
      }

      setError(null);
      fetchUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update user');
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Users</h1>
          <p className="text-sm text-muted-foreground">
            Manage user accounts and their roles
          </p>
        </div>
        <Button onClick={() => setShowCreateModal(true)} size="sm" className="h-8">
          <UserPlus className="h-4 w-4 mr-1.5" />
          Add User
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
              <TableHead className="w-[200px]">Username</TableHead>
              <TableHead className="w-[100px]">Role</TableHead>
              <TableHead className="w-[80px]">Auth</TableHead>
              <TableHead className="w-[80px]">Status</TableHead>
              <TableHead className="w-[150px]">Created</TableHead>
              <TableHead className="w-[150px]">Updated</TableHead>
              <TableHead className="w-[80px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                  Loading users...
                </TableCell>
              </TableRow>
            ) : users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                  No users found
                </TableCell>
              </TableRow>
            ) : (
              users.map((user) => (
                <TableRow key={user.id} className={user.isDisabled ? 'opacity-60' : ''}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      {user.username}
                      {user.id === currentUser?.id && (
                        <span className="text-xs text-muted-foreground">(you)</span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                      {user.role}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {user.authProvider === 'oidc' ? 'SSO' : 'Local'}
                  </TableCell>
                  <TableCell className="text-sm">
                    {user.isDisabled ? (
                      <span className="text-red-600 dark:text-red-400">Disabled</span>
                    ) : (
                      <span className="text-muted-foreground">Active</span>
                    )}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {dayjs(user.createdAt).format('MMM D, YYYY HH:mm')}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {dayjs(user.updatedAt).format('MMM D, YYYY HH:mm')}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditingUser(user)}>
                          <Pencil className="h-4 w-4 mr-2" />
                          Edit
                        </DropdownMenuItem>
                        {isAdmin && (
                          <DropdownMenuItem onClick={() => setResetPasswordUser(user)}>
                            <Key className="h-4 w-4 mr-2" />
                            Reset Password
                          </DropdownMenuItem>
                        )}
                        {isAdmin && user.id !== currentUser?.id && (
                          <DropdownMenuItem onClick={() => handleToggleDisabled(user)}>
                            {user.isDisabled ? (
                              <>
                                <UserCheck className="h-4 w-4 mr-2" />
                                Enable
                              </>
                            ) : (
                              <>
                                <Ban className="h-4 w-4 mr-2" />
                                Disable
                              </>
                            )}
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem
                          onClick={() => setDeletingUser(user)}
                          className="text-destructive"
                          disabled={user.id === currentUser?.id}
                        >
                          <Trash2 className="h-4 w-4 mr-2" />
                          Delete
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

      {/* Create User Modal */}
      <UserFormModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSuccess={() => {
          setShowCreateModal(false);
          fetchUsers();
        }}
      />

      {/* Edit User Modal */}
      <UserFormModal
        open={!!editingUser}
        user={editingUser || undefined}
        onClose={() => setEditingUser(null)}
        onSuccess={() => {
          setEditingUser(null);
          fetchUsers();
        }}
      />

      {/* Reset Password Modal */}
      <ResetPasswordModal
        open={!!resetPasswordUser}
        user={resetPasswordUser || undefined}
        onClose={() => setResetPasswordUser(null)}
      />

      {/* Delete Confirmation */}
      <ConfirmModal
        title="Delete User"
        buttonText="Delete"
        visible={!!deletingUser}
        dismissModal={() => setDeletingUser(null)}
        onSubmit={handleDeleteUser}
      >
        <p>Are you sure you want to delete user &quot;{deletingUser?.username}&quot;? This action cannot be undone.</p>
      </ConfirmModal>
    </div>
  );
}