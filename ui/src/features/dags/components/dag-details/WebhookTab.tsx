/**
 * WebhookTab component displays webhook configuration for a DAG.
 * Webhooks are managed separately from DAG configuration.
 *
 * @module features/dags/components/dag-details
 */
import {
  AlertTriangle,
  Check,
  Copy,
  Loader2,
  Plus,
  RefreshCw,
  Terminal,
  Trash2,
  Webhook,
  WebhookOff,
} from 'lucide-react';
import { useCallback, useContext, useEffect, useState } from 'react';
import { components } from '../../../../api/v1/schema';
import { Button } from '../../../../components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '../../../../components/ui/card';
import { Switch } from '../../../../components/ui/switch';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { TOKEN_KEY } from '../../../../contexts/AuthContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import dayjs from '../../../../lib/dayjs';
import ConfirmModal from '../../../../ui/ConfirmModal';

type WebhookDetails = components['schemas']['WebhookDetails'];

interface WebhookTabProps {
  fileName: string;
}

interface CopyButtonProps {
  copied: boolean;
  onCopy: () => void;
  label?: string;
}

function CopyButton({ copied, onCopy, label }: CopyButtonProps) {
  return (
    <Button
      variant="ghost"
      size="sm"
      className="h-6 px-2 text-muted-foreground hover:text-foreground"
      onClick={onCopy}
    >
      {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
      {label && <span className="ml-1 text-xs">{label}</span>}
    </Button>
  );
}

function WebhookTab({ fileName }: WebhookTabProps) {
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);

  // State
  const [webhook, setWebhook] = useState<WebhookDetails | null>(null);
  const [newToken, setNewToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isActioning, setIsActioning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showToggleConfirm, setShowToggleConfirm] = useState(false);
  const [pendingToggleState, setPendingToggleState] = useState<boolean | null>(
    null
  );

  // Copy states
  const [copiedUrl, setCopiedUrl] = useState(false);
  const [copiedToken, setCopiedToken] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  // Construct webhook URL (include remoteNode if not local)
  const remoteNode = appBarContext.selectedRemoteNode;
  const webhookUrl =
    remoteNode && remoteNode !== 'local'
      ? `${window.location.origin}/api/v1/webhooks/${encodeURIComponent(fileName)}?remoteNode=${encodeURIComponent(remoteNode)}`
      : `${window.location.origin}/api/v1/webhooks/${encodeURIComponent(fileName)}`;

  // API helpers
  const getAuthHeaders = useCallback(() => {
    const token = localStorage.getItem(TOKEN_KEY);
    return {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    };
  }, []);

  const getRemoteNodeParam = useCallback(() => {
    return appBarContext.selectedRemoteNode || 'local';
  }, [appBarContext.selectedRemoteNode]);

  // Fetch webhook
  const fetchWebhook = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const remoteNode = getRemoteNodeParam();
      const response = await fetch(
        `${config.apiURL}/dags/${encodeURIComponent(fileName)}/webhook?remoteNode=${remoteNode}`,
        { headers: getAuthHeaders() }
      );

      if (response.status === 404) {
        setWebhook(null);
        return;
      }

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to fetch webhook');
      }

      const data = await response.json();
      setWebhook(data);
    } catch (err) {
      if (err instanceof Error && err.message.includes('404')) {
        setWebhook(null);
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load webhook');
      }
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL, fileName, getAuthHeaders, getRemoteNodeParam]);

  useEffect(() => {
    fetchWebhook();
  }, [fetchWebhook]);

  // Create webhook
  const handleCreate = async () => {
    try {
      setIsActioning(true);
      setError(null);
      const remoteNode = getRemoteNodeParam();
      const response = await fetch(
        `${config.apiURL}/dags/${encodeURIComponent(fileName)}/webhook?remoteNode=${remoteNode}`,
        {
          method: 'POST',
          headers: getAuthHeaders(),
        }
      );

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to create webhook');
      }

      const data = await response.json();
      setWebhook(data.webhook);
      setNewToken(data.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create webhook');
    } finally {
      setIsActioning(false);
    }
  };

  // Delete webhook
  const handleDelete = async () => {
    try {
      setIsActioning(true);
      setError(null);
      const remoteNode = getRemoteNodeParam();
      const response = await fetch(
        `${config.apiURL}/dags/${encodeURIComponent(fileName)}/webhook?remoteNode=${remoteNode}`,
        {
          method: 'DELETE',
          headers: getAuthHeaders(),
        }
      );

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to delete webhook');
      }

      setWebhook(null);
      setNewToken(null);
      setShowDeleteConfirm(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete webhook');
    } finally {
      setIsActioning(false);
    }
  };

  // Regenerate token
  const handleRegenerate = async () => {
    try {
      setIsActioning(true);
      setError(null);
      const remoteNode = getRemoteNodeParam();
      const response = await fetch(
        `${config.apiURL}/dags/${encodeURIComponent(fileName)}/webhook/regenerate?remoteNode=${remoteNode}`,
        {
          method: 'POST',
          headers: getAuthHeaders(),
        }
      );

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to regenerate token');
      }

      const data = await response.json();
      setWebhook(data.webhook);
      setNewToken(data.token);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to regenerate token'
      );
    } finally {
      setIsActioning(false);
    }
  };

  // Toggle enabled - show confirmation first
  const handleToggleClick = (enabled: boolean) => {
    setPendingToggleState(enabled);
    setShowToggleConfirm(true);
  };

  const handleToggleConfirm = async () => {
    if (pendingToggleState === null) return;
    try {
      setError(null);
      const remoteNode = getRemoteNodeParam();
      const response = await fetch(
        `${config.apiURL}/dags/${encodeURIComponent(fileName)}/webhook/toggle?remoteNode=${remoteNode}`,
        {
          method: 'POST',
          headers: getAuthHeaders(),
          body: JSON.stringify({ enabled: pendingToggleState }),
        }
      );

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to toggle webhook');
      }

      const data = await response.json();
      setWebhook(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to toggle webhook');
    } finally {
      setShowToggleConfirm(false);
      setPendingToggleState(null);
    }
  };

  const handleToggleCancel = () => {
    setShowToggleConfirm(false);
    setPendingToggleState(null);
  };

  // Copy handler
  const handleCopy = async (text: string, setCopied: (v: boolean) => void) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API might not be available
    }
  };

  // Dismiss token display
  const handleDismissToken = () => {
    setNewToken(null);
  };

  // Loading state
  if (isLoading) {
    return (
      <Card className="max-w-xl gap-0 py-0">
        <CardHeader className="pb-2 px-4 pt-3">
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            <CardTitle className="text-sm">Loading webhook...</CardTitle>
          </div>
        </CardHeader>
      </Card>
    );
  }

  // Error state
  if (error) {
    return (
      <Card className="max-w-xl gap-0 py-0 border-destructive/50">
        <CardHeader className="pb-2 px-4 pt-3">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 text-destructive" />
            <CardTitle className="text-sm text-destructive">Error</CardTitle>
          </div>
          <CardDescription className="text-xs text-destructive">
            {error}
          </CardDescription>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-2">
          <Button variant="outline" size="sm" onClick={fetchWebhook}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  // Token reveal after create/regenerate
  if (newToken) {
    return (
      <Card className="max-w-xl gap-0 py-0">
        <CardHeader className="pb-2 px-4 pt-3">
          <div className="flex items-center gap-2">
            <Webhook className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm">Webhook Created</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-0 space-y-3">
          <div className="p-3 bg-warning/10 border border-warning/20 rounded-md">
            <p className="text-sm text-warning-foreground">
              Copy this token now. You won&apos;t be able to see it again!
            </p>
          </div>
          <div className="flex items-center gap-2">
            <code className="flex-1 p-2 text-xs bg-muted rounded-md break-all font-mono">
              {newToken}
            </code>
            <Button
              variant="outline"
              size="icon"
              onClick={() => handleCopy(newToken, setCopiedToken)}
            >
              {copiedToken ? (
                <Check className="h-4 w-4" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>
          <Button variant="default" size="sm" onClick={handleDismissToken}>
            Done
          </Button>
        </CardContent>
      </Card>
    );
  }

  // No webhook configured
  if (!webhook) {
    return (
      <Card className="max-w-xl gap-0 py-0">
        <CardHeader className="pb-2 px-4 pt-3">
          <div className="flex items-center gap-2">
            <WebhookOff className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm">No Webhook Configured</CardTitle>
          </div>
          <CardDescription className="text-xs">
            Create a webhook to trigger this DAG via HTTP
          </CardDescription>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-3">
          <Button
            variant="default"
            size="sm"
            onClick={handleCreate}
            disabled={isActioning}
          >
            {isActioning ? (
              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
            ) : (
              <Plus className="h-4 w-4 mr-1" />
            )}
            Create Webhook
          </Button>
        </CardContent>
      </Card>
    );
  }

  // Generate example curl command
  const tokenForCurl = '<YOUR_TOKEN>';
  const curlExample = `curl -X POST "${webhookUrl}" \\
  -H "Authorization: Bearer ${tokenForCurl}" \\
  -H "Content-Type: application/json" \\
  -d '{"dagRunId": "my-unique-id", "payload": {"key": "value"}}'`;

  // Webhook configured
  return (
    <div className="space-y-3 max-w-2xl">
      {/* Status Card */}
      <Card className="gap-0 py-0">
        <CardHeader className="pb-0 px-4 pt-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Webhook className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm">Webhook</CardTitle>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">
                {webhook.enabled ? 'Enabled' : 'Disabled'}
              </span>
              <Switch
                checked={webhook.enabled}
                onCheckedChange={handleToggleClick}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-2 space-y-3">
          {/* Endpoint */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-muted-foreground">Endpoint</span>
              <CopyButton
                copied={copiedUrl}
                onCopy={() => handleCopy(webhookUrl, setCopiedUrl)}
              />
            </div>
            <div className="px-3 py-2 bg-accent rounded-md text-xs font-mono border overflow-x-auto">
              <span className="text-muted-foreground">POST</span>{' '}
              <span>{webhookUrl}</span>
            </div>
          </div>

          {/* Token prefix */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-muted-foreground">Token</span>
            </div>
            <div className="px-3 py-2 bg-accent rounded-md text-xs font-mono border">
              <span>{webhook.tokenPrefix}</span>
              <span className="text-muted-foreground">{'*'.repeat(32)}</span>
            </div>
          </div>

          {/* Metadata */}
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <div>
              Created: {dayjs(webhook.createdAt).format('MMM D, YYYY HH:mm')}
            </div>
            {webhook.lastUsedAt && (
              <div>
                Last triggered:{' '}
                {dayjs(webhook.lastUsedAt).format('MMM D, YYYY HH:mm')}
              </div>
            )}
            {webhook.createdBy && <div>By: {webhook.createdBy}</div>}
          </div>

          {/* Actions */}
          <div className="flex gap-2 pt-1">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRegenerate}
              disabled={isActioning}
            >
              {isActioning ? (
                <Loader2 className="h-3.5 w-3.5 mr-1 animate-spin" />
              ) : (
                <RefreshCw className="h-3.5 w-3.5 mr-1" />
              )}
              Regenerate Token
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowDeleteConfirm(true)}
              className="text-destructive hover:text-destructive"
              disabled={isActioning}
            >
              <Trash2 className="h-3.5 w-3.5 mr-1" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Example Card */}
      <Card className="gap-0 py-0">
        <CardHeader className="pb-0 px-4 pt-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
              <CardTitle className="text-sm">Example Request</CardTitle>
            </div>
            <CopyButton
              copied={copiedCurl}
              onCopy={() => handleCopy(curlExample, setCopiedCurl)}
              label="Copy"
            />
          </div>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-2">
          <pre className="px-3 py-2 bg-accent rounded-md text-xs font-mono border overflow-x-auto whitespace-pre-wrap">
            {curlExample}
          </pre>
          <ul className="mt-2 text-xs text-muted-foreground space-y-1">
            <li>
              <code className="bg-accent px-1 rounded-md border">payload</code>{' '}
              is available as{' '}
              <code className="bg-accent px-1 rounded-md border">
                WEBHOOK_PAYLOAD
              </code>{' '}
              env var.
            </li>
            <li>
              <code className="bg-accent px-1 rounded-md border">dagRunId</code>{' '}
              (optional) can be used as an idempotency key.
            </li>
          </ul>
        </CardContent>
      </Card>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        title="Delete Webhook"
        buttonText="Delete"
        visible={showDeleteConfirm}
        dismissModal={() => setShowDeleteConfirm(false)}
        onSubmit={handleDelete}
      >
        <p>
          Are you sure you want to delete this webhook? Any applications using
          this webhook token will immediately lose access.
        </p>
      </ConfirmModal>

      {/* Toggle Confirmation Modal */}
      <ConfirmModal
        title={pendingToggleState ? 'Enable Webhook' : 'Disable Webhook'}
        buttonText={pendingToggleState ? 'Enable' : 'Disable'}
        visible={showToggleConfirm}
        dismissModal={handleToggleCancel}
        onSubmit={handleToggleConfirm}
      >
        <p>
          Are you sure you want to {pendingToggleState ? 'enable' : 'disable'}{' '}
          this webhook?
        </p>
      </ConfirmModal>
    </div>
  );
}

export default WebhookTab;
