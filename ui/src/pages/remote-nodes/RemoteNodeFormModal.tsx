import { useState, useEffect, useContext } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { TOKEN_KEY } from '@/contexts/AuthContext';
import { components, CreateRemoteNodeRequestAuthType } from '@/api/v1/schema';
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
import { Switch } from '@/components/ui/switch';
import { AlertCircle, Check, Plus, X } from 'lucide-react';

type RemoteNodeResponse = components['schemas']['RemoteNodeResponse'];

type RemoteNodeFormModalProps = {
  open: boolean;
  node?: RemoteNodeResponse;
  onClose: () => void;
  onSuccess: () => void;
};

export function RemoteNodeFormModal({
  open,
  node,
  onClose,
  onSuccess,
}: RemoteNodeFormModalProps) {
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);
  const isEditing = !!node;

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [apiBaseUrl, setApiBaseUrl] = useState('');
  const [authType, setAuthType] = useState<string>('none');
  const [basicAuthUsername, setBasicAuthUsername] = useState('');
  const [basicAuthPassword, setBasicAuthPassword] = useState('');
  const [authToken, setAuthToken] = useState('');
  const [skipTlsVerify, setSkipTlsVerify] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (node) {
      setName(node.name);
      setDescription(node.description || '');
      setApiBaseUrl(node.apiBaseUrl);
      setAuthType(node.authType);
      setSkipTlsVerify(node.skipTlsVerify || false);
      setBasicAuthUsername('');
      setBasicAuthPassword('');
      setAuthToken('');
    } else {
      setName('');
      setDescription('');
      setApiBaseUrl('');
      setAuthType('none');
      setBasicAuthUsername('');
      setBasicAuthPassword('');
      setAuthToken('');
      setSkipTlsVerify(false);
    }
    setError(null);
  }, [node, open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!name.trim()) {
      setError('Name is required');
      return;
    }

    if (!apiBaseUrl.trim()) {
      setError('API Base URL is required');
      return;
    }

    if (!/^https?:\/\//.test(apiBaseUrl)) {
      setError('API Base URL must start with http:// or https://');
      return;
    }

    setIsLoading(true);

    try {
      const token = localStorage.getItem(TOKEN_KEY);
      if (!token) {
        throw new Error('Not authenticated');
      }
      const remoteNode = encodeURIComponent(
        appBarContext.selectedRemoteNode || 'local'
      );

      if (isEditing) {
        const body: Record<string, unknown> = {
          name,
          description,
          apiBaseUrl,
          authType,
          skipTlsVerify,
        };
        if (authType === 'basic') {
          if (basicAuthUsername) body.basicAuthUsername = basicAuthUsername;
          if (basicAuthPassword) body.basicAuthPassword = basicAuthPassword;
        } else if (authType === 'token') {
          if (authToken) body.authToken = authToken;
        }

        const response = await fetch(
          `${config.apiURL}/remote-nodes/${node.id}?remoteNode=${remoteNode}`,
          {
            method: 'PATCH',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify(body),
          }
        );

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(data.message || 'Failed to update remote node');
        }
      } else {
        const body: Record<string, unknown> = {
          name,
          description,
          apiBaseUrl,
          authType: authType as CreateRemoteNodeRequestAuthType,
          skipTlsVerify,
        };
        if (authType === 'basic') {
          body.basicAuthUsername = basicAuthUsername;
          body.basicAuthPassword = basicAuthPassword;
        } else if (authType === 'token') {
          body.authToken = authToken;
        }

        const response = await fetch(
          `${config.apiURL}/remote-nodes?remoteNode=${remoteNode}`,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify(body),
          }
        );

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(data.message || 'Failed to create remote node');
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
          <DialogTitle>
            {isEditing ? 'Edit Remote Node' : 'Add Remote Node'}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="name" className="text-sm">
              Name
            </Label>
            <Input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              autoComplete="off"
              className="h-9"
              placeholder="e.g. production-server"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="description" className="text-sm">
              Description
            </Label>
            <Input
              id="description"
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              autoComplete="off"
              className="h-9"
              placeholder="Optional description"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="apiBaseUrl" className="text-sm">
              API Base URL
            </Label>
            <Input
              id="apiBaseUrl"
              type="text"
              value={apiBaseUrl}
              onChange={(e) => setApiBaseUrl(e.target.value)}
              required
              autoComplete="off"
              className="h-9"
              placeholder="https://dagu.example.com:8080"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="authType" className="text-sm">
              Authentication
            </Label>
            <Select value={authType} onValueChange={setAuthType}>
              <SelectTrigger className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">None</SelectItem>
                <SelectItem value="basic">Basic Auth</SelectItem>
                <SelectItem value="token">Bearer Token</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {authType === 'basic' && (
            <>
              <div className="space-y-1.5">
                <Label htmlFor="basicAuthUsername" className="text-sm">
                  Username
                </Label>
                <Input
                  id="basicAuthUsername"
                  type="text"
                  value={basicAuthUsername}
                  onChange={(e) => setBasicAuthUsername(e.target.value)}
                  autoComplete="off"
                  className="h-9"
                  placeholder={isEditing ? '(unchanged)' : ''}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="basicAuthPassword" className="text-sm">
                  Password
                </Label>
                <Input
                  id="basicAuthPassword"
                  type="password"
                  value={basicAuthPassword}
                  onChange={(e) => setBasicAuthPassword(e.target.value)}
                  autoComplete="new-password"
                  className="h-9"
                  placeholder={isEditing ? '(unchanged)' : ''}
                />
              </div>
            </>
          )}

          {authType === 'token' && (
            <div className="space-y-1.5">
              <Label htmlFor="authToken" className="text-sm">
                Token
              </Label>
              <Input
                id="authToken"
                type="password"
                value={authToken}
                onChange={(e) => setAuthToken(e.target.value)}
                autoComplete="off"
                className="h-9"
                placeholder={isEditing ? '(unchanged)' : ''}
              />
            </div>
          )}

          <div className="flex items-center gap-2">
            <Switch
              id="skipTlsVerify"
              checked={skipTlsVerify}
              onCheckedChange={setSkipTlsVerify}
            />
            <Label htmlFor="skipTlsVerify" className="text-sm cursor-pointer">
              Skip TLS verification
            </Label>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={onClose}>
              <X className="h-4 w-4" />
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isEditing ? (
                <Check className="h-4 w-4" />
              ) : (
                <Plus className="h-4 w-4" />
              )}
              {isLoading ? 'Saving...' : isEditing ? 'Update' : 'Create'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
