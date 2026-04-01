import { useMemo, useState } from 'react';
import {
  CheckCircle2,
  Copy,
  ExternalLink,
  Loader2,
  RefreshCcw,
  ShieldCheck,
  Unplug,
} from 'lucide-react';
import { components } from '@/api/v1/schema';
import { Badge } from '@/components/ui/badge';
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
import { Textarea } from '@/components/ui/textarea';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { cn } from '@/lib/utils';

type AgentAuthProviderStatus = components['schemas']['AgentAuthProviderStatus'];
type StartAgentAuthProviderLoginResponse = components['schemas']['StartAgentAuthProviderLoginResponse'];
type CompleteAgentAuthProviderLoginRequest = components['schemas']['CompleteAgentAuthProviderLoginRequest'];

interface ProviderAuthCardProps {
  provider: AgentAuthProviderStatus;
  isLoading?: boolean;
  compact?: boolean;
  className?: string;
  onStartLogin: (providerId: string) => Promise<StartAgentAuthProviderLoginResponse>;
  onCompleteLogin: (
    providerId: string,
    body: CompleteAgentAuthProviderLoginRequest
  ) => Promise<AgentAuthProviderStatus | null>;
  onDisconnect: (providerId: string) => Promise<void>;
}

function formatTimestamp(value?: string): string | null {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return date.toLocaleString();
}

function maskAccountID(accountId?: string): string | null {
  if (!accountId) {
    return null;
  }
  if (accountId.length <= 10) {
    return accountId;
  }
  return `${accountId.slice(0, 6)}...${accountId.slice(-4)}`;
}

export function ProviderAuthCard({
  provider,
  isLoading = false,
  compact = false,
  className,
  onStartLogin,
  onCompleteLogin,
  onDisconnect,
}: ProviderAuthCardProps) {
  const { showToast } = useSimpleToast();
  const [loginFlow, setLoginFlow] = useState<StartAgentAuthProviderLoginResponse | null>(null);
  const [redirectUrl, setRedirectUrl] = useState('');
  const [code, setCode] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [isStartingLogin, setIsStartingLogin] = useState(false);
  const [isCompletingLogin, setIsCompletingLogin] = useState(false);
  const [isDisconnecting, setIsDisconnecting] = useState(false);

  const statusMeta = useMemo(() => {
    if (!provider.connected) {
      return {
        label: 'Not Connected',
        variant: 'warning' as const,
        description: 'Connect your ChatGPT subscription before using this provider.',
      };
    }
    if (provider.expiresAt) {
      const expiry = new Date(provider.expiresAt);
      if (!Number.isNaN(expiry.getTime()) && expiry.getTime() <= Date.now()) {
        return {
          label: 'Expired',
          variant: 'warning' as const,
          description: 'The stored session has expired. Reconnect to continue using this provider.',
        };
      }
    }
    return {
      label: 'Connected',
      variant: 'success' as const,
      description: 'The selected node has a valid ChatGPT subscription session.',
    };
  }, [provider.connected, provider.expiresAt]);

  const expiryLabel = formatTimestamp(provider.expiresAt);
  const maskedAccountID = maskAccountID(provider.accountId);
  const hasPendingInput = redirectUrl.trim() !== '' || code.trim() !== '';

  const resetDialog = () => {
    setDialogOpen(false);
    setLoginFlow(null);
    setRedirectUrl('');
    setCode('');
    setActionError(null);
  };

  const handleCopy = async (value: string, successMessage: string) => {
    try {
      await navigator.clipboard.writeText(value);
      showToast(successMessage);
    } catch {
      setActionError('Failed to copy to clipboard');
    }
  };

  const handleStartLogin = async () => {
    setActionError(null);
    setIsStartingLogin(true);
    try {
      const flow = await onStartLogin(provider.id);
      setLoginFlow(flow);
      setDialogOpen(true);
      window.open(flow.authUrl, '_blank', 'noopener,noreferrer');
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to start provider login');
    } finally {
      setIsStartingLogin(false);
    }
  };

  const handleCompleteLogin = async () => {
    if (!loginFlow) {
      return;
    }
    setActionError(null);
    setIsCompletingLogin(true);
    try {
      await onCompleteLogin(provider.id, {
        flowId: loginFlow.flowId,
        redirectUrl: redirectUrl.trim() || undefined,
        code: code.trim() || undefined,
      });
      showToast(`${provider.name} connected`);
      resetDialog();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to complete provider login');
    } finally {
      setIsCompletingLogin(false);
    }
  };

  const handleDisconnect = async () => {
    setActionError(null);
    setIsDisconnecting(true);
    try {
      await onDisconnect(provider.id);
      showToast(`${provider.name} disconnected`);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to disconnect provider');
    } finally {
      setIsDisconnecting(false);
    }
  };

  return (
    <>
      <div
        className={cn(
          'rounded-md border border-border/60 bg-background/60',
          compact ? 'p-3 space-y-3' : 'card-obsidian p-4 space-y-4',
          className
        )}
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-muted-foreground" />
              <p className="text-sm font-medium">{provider.name}</p>
              <Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              {statusMeta.description}
            </p>
            <p className="text-xs text-muted-foreground">
              Uses your ChatGPT subscription login. API keys and custom base URLs are not used.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              className="h-8"
              onClick={handleStartLogin}
              disabled={isLoading || isStartingLogin || isDisconnecting}
            >
              {isStartingLogin ? (
                <>
                  <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                  Opening...
                </>
              ) : (
                <>
                  <ExternalLink className="h-4 w-4 mr-1.5" />
                  {provider.connected ? 'Reconnect' : 'Connect'}
                </>
              )}
            </Button>
            {provider.connected && (
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-8"
                onClick={handleDisconnect}
                disabled={isLoading || isStartingLogin || isDisconnecting}
              >
                {isDisconnecting ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                    Disconnecting...
                  </>
                ) : (
                  <>
                    <Unplug className="h-4 w-4 mr-1.5" />
                    Disconnect
                  </>
                )}
              </Button>
            )}
          </div>
        </div>

        <div className={cn('grid gap-3', compact ? 'sm:grid-cols-1' : 'sm:grid-cols-3')}>
          <div className="space-y-1">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              Account
            </p>
            <p className="text-sm">
              {maskedAccountID || 'Not connected'}
            </p>
          </div>
          <div className="space-y-1">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              Expires
            </p>
            <p className="text-sm">
              {expiryLabel || 'Managed automatically'}
            </p>
          </div>
          <div className="space-y-1">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              Refresh
            </p>
            <p className="text-sm flex items-center gap-2">
              {provider.canRefresh ? (
                <>
                  <RefreshCcw className="h-3.5 w-3.5 text-muted-foreground" />
                  Supported
                </>
              ) : (
                'Manual reconnect only'
              )}
            </p>
          </div>
        </div>

        {actionError && !dialogOpen && (
          <div className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {actionError}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open) => {
        if (!open && !isCompletingLogin) {
          resetDialog();
          return;
        }
        setDialogOpen(open);
      }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Connect {provider.name}</DialogTitle>
            <DialogDescription className="text-sm text-muted-foreground">
              Authenticate in your browser, then paste the failed localhost redirect URL or the raw authorization code here.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {actionError && (
              <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {actionError}
              </div>
            )}

            <div className="rounded-md border border-border/60 bg-muted/30 p-3 text-sm">
              <div className="flex items-start gap-2">
                <CheckCircle2 className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div className="space-y-1">
                  <p>1. Open the login page in your browser and finish the ChatGPT sign-in flow.</p>
                  <p>2. When the browser redirects to `http://localhost:1455/auth/callback`, copy the full URL from the address bar.</p>
                  <p>3. Paste that URL below. If you only have the code value, paste it into the authorization code field instead.</p>
                </div>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="provider-auth-url" className="text-sm">Authorization URL</Label>
              <div className="flex gap-2">
                <Input
                  id="provider-auth-url"
                  value={loginFlow?.authUrl || ''}
                  readOnly
                  className="font-mono text-xs"
                />
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-9 shrink-0"
                  onClick={() => loginFlow && handleCopy(loginFlow.authUrl, 'Authorization URL copied')}
                  disabled={!loginFlow}
                >
                  <Copy className="h-4 w-4" />
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-9 shrink-0"
                  onClick={() => loginFlow && window.open(loginFlow.authUrl, '_blank', 'noopener,noreferrer')}
                  disabled={!loginFlow}
                >
                  <ExternalLink className="h-4 w-4" />
                </Button>
              </div>
              {loginFlow?.instructions && (
                <p className="text-xs text-muted-foreground">{loginFlow.instructions}</p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="provider-redirect-url" className="text-sm">Redirect URL</Label>
              <Textarea
                id="provider-redirect-url"
                value={redirectUrl}
                onChange={(event) => setRedirectUrl(event.target.value)}
                placeholder="http://localhost:1455/auth/callback?code=...&state=..."
                className="min-h-24 font-mono text-xs"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="provider-auth-code" className="text-sm">Authorization Code</Label>
              <Input
                id="provider-auth-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder="Paste only the code if you do not have the full redirect URL"
                className="font-mono text-xs"
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={resetDialog} disabled={isCompletingLogin}>
              Cancel
            </Button>
            <Button
              type="button"
              onClick={handleCompleteLogin}
              disabled={isCompletingLogin || !hasPendingInput}
            >
              {isCompletingLogin ? (
                <>
                  <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                  Connecting...
                </>
              ) : (
                'Complete Connection'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
