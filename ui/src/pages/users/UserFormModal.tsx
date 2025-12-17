import { useState, useEffect } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { components } from '@/api/v2/schema';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { AlertCircle } from 'lucide-react';

type User = components['schemas']['User'];

type UserFormModalProps = {
  open: boolean;
  user?: User;
  onClose: () => void;
  onSuccess: () => void;
};

const ROLES = [
  { value: 'admin', label: 'Admin', description: 'Full access including user management' },
  { value: 'manager', label: 'Manager', description: 'DAG create/edit/delete and execution' },
  { value: 'operator', label: 'Operator', description: 'DAG execution only' },
  { value: 'viewer', label: 'Viewer', description: 'Read-only access' },
] as const;

/**
 * Render a modal dialog that provides a form to create a new user or edit an existing one.
 *
 * @param props.open - Whether the modal is open.
 * @param props.user - Existing user to edit; when undefined the form operates in create mode.
 * @param props.onClose - Callback invoked when the modal is closed.
 * @param props.onSuccess - Callback invoked after a successful create or update operation.
 * @returns The modal JSX element containing the user form.
 */
export function UserFormModal({ open, user, onClose, onSuccess }: UserFormModalProps) {
  const config = useConfig();
  const isEditing = !!user;

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<string>('viewer');
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (user) {
      setUsername(user.username);
      setRole(user.role);
      setPassword('');
    } else {
      setUsername('');
      setPassword('');
      setRole('viewer');
    }
    setError(null);
  }, [user, open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!isEditing && password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    setIsLoading(true);

    try {
      const token = localStorage.getItem('dagu_auth_token');

      if (isEditing) {
        // Update user
        const response = await fetch(`${config.apiURL}/users/${user.id}`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ username, role }),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(data.message || 'Failed to update user');
        }
      } else {
        // Create user
        const response = await fetch(`${config.apiURL}/users`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ username, password, role }),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(data.message || 'Failed to create user');
        }
      }

      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Operation failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit User' : 'Create User'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="username" className="text-sm">
              Username
            </Label>
            <Input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="off"
              className="h-9"
            />
          </div>

          {!isEditing && (
            <div className="space-y-1.5">
              <Label htmlFor="password" className="text-sm">
                Password
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="new-password"
                className="h-9"
                placeholder="Minimum 8 characters"
              />
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="role" className="text-sm">
              Role
            </Label>
            <Select value={role} onValueChange={setRole}>
              <SelectTrigger className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((r) => (
                  <SelectItem key={r.value} value={r.value}>
                    <div className="flex flex-col">
                      <span>{r.label}</span>
                      <span className="text-xs text-muted-foreground">{r.description}</span>
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose} className="h-8">
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading} className="h-8">
              {isLoading ? 'Saving...' : isEditing ? 'Update' : 'Create'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}