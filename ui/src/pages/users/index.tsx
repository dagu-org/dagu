import { useState, useEffect, useCallback, useContext } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { useAuth, TOKEN_KEY } from '@/contexts/AuthContext';
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
import { UserPlus, MoreHorizontal, Pencil, Trash2, Key, Shield } from 'lucide-react';
import { UserFormModal } from './UserFormModal';
import { ResetPasswordModal } from './ResetPasswordModal';
import ConfirmModal from '@/ui/ConfirmModal';
import dayjs from '@/lib/dayjs';

type User = components['schemas']['User'];

export default function UsersPage() {
  const config = useConfig();
  const { user: currentUser } = useAuth();
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
      const response = await fetch(`${config.apiURL}/users`, {
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
  }, [config.apiURL]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleDeleteUser = async () => {
    if (!deletingUser) return;

    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const response = await fetch(`${config.apiURL}/users/${deletingUser.id}`, {
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

  const getRoleBadgeColor = (role: string) => {
    switch (role) {
      case 'admin':
        return 'bg-red-500/20 text-red-600 dark:text-red-400';
      case 'manager':
        return 'bg-blue-500/20 text-blue-600 dark:text-blue-400';
      case 'operator':
        return 'bg-green-500/20 text-green-600 dark:text-green-400';
      case 'viewer':
        return 'bg-gray-500/20 text-gray-600 dark:text-gray-400';
      default:
        return 'bg-gray-500/20 text-gray-600 dark:text-gray-400';
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
              <TableHead className="w-[120px]">Role</TableHead>
              <TableHead className="w-[180px]">Created</TableHead>
              <TableHead className="w-[180px]">Updated</TableHead>
              <TableHead className="w-[80px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                  Loading users...
                </TableCell>
              </TableRow>
            ) : users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                  No users found
                </TableCell>
              </TableRow>
            ) : (
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      {user.username}
                      {user.id === currentUser?.id && (
                        <span className="text-xs text-muted-foreground">(you)</span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <Shield className="h-3.5 w-3.5 text-muted-foreground" />
                      <span
                        className={`text-xs px-1.5 py-0.5 rounded-full capitalize ${getRoleBadgeColor(user.role)}`}
                      >
                        {user.role}
                      </span>
                    </div>
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
                        <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditingUser(user)}>
                          <Pencil className="h-4 w-4 mr-2" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => setResetPasswordUser(user)}>
                          <Key className="h-4 w-4 mr-2" />
                          Reset Password
                        </DropdownMenuItem>
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
