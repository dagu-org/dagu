import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import { TOKEN_KEY } from '@/contexts/AuthContext';
import { CheckCircle2, Shield, XCircle } from 'lucide-react';
import { useContext, useEffect, useState } from 'react';

export default function LicensePage() {
  const config = useConfig();
  const { license } = config;
  const appBarContext = useContext(AppBarContext);

  const [key, setKey] = useState('');
  const [activating, setActivating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    appBarContext.setTitle('License');
  }, [appBarContext]);

  async function handleActivate() {
    if (!key.trim()) return;
    setActivating(true);
    setError(null);
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = appBarContext.selectedRemoteNode || 'local';
      const response = await fetch(
        `${config.apiURL}/license/activate?remoteNode=${remoteNode}`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ key: key.trim() }),
        }
      );
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Activation failed');
      }
      // Reload to pick up new license state from server-rendered config
      window.location.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Activation failed');
    } finally {
      setActivating(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <div>
        <h1 className="text-lg font-semibold">License</h1>
        <p className="text-sm text-muted-foreground">
          View license status and activate a Dagu Pro license key.
        </p>
      </div>

      {/* Current status */}
      <div className="card-obsidian p-4 space-y-3">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Shield className="h-4 w-4" />
          Current License
        </div>
        <div className="grid grid-cols-[120px_1fr] gap-y-2 text-sm">
          <span className="text-muted-foreground">Status</span>
          <span className="flex items-center gap-1.5">
            {license.valid ? (
              <>
                <CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
                Active
              </>
            ) : license.gracePeriod ? (
              <>
                <XCircle className="h-3.5 w-3.5 text-amber-500" />
                Grace Period
              </>
            ) : license.community ? (
              'Community Edition'
            ) : (
              <>
                <XCircle className="h-3.5 w-3.5 text-red-500" />
                Inactive
              </>
            )}
          </span>

          <span className="text-muted-foreground">Plan</span>
          <span className="capitalize">{license.plan || 'community'}</span>

          <span className="text-muted-foreground">Features</span>
          <span>
            {license.features.length > 0
              ? license.features.join(', ')
              : 'None'}
          </span>

          {license.expiry && (
            <>
              <span className="text-muted-foreground">Expires</span>
              <span>{new Date(license.expiry).toLocaleDateString()}</span>
            </>
          )}
        </div>
      </div>

      {/* Activation form */}
      <div className="card-obsidian p-4 space-y-3">
        <div className="text-sm font-medium">Activate License Key</div>
        <div className="flex gap-2">
          <Input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="DAGU-XXXX-XXXX-XXXX-XXXX"
            className="font-mono text-sm h-8"
          />
          <Button
            size="sm"
            className="h-8 flex-shrink-0"
            onClick={handleActivate}
            disabled={activating || !key.trim()}
          >
            {activating ? 'Activating...' : 'Activate'}
          </Button>
        </div>
        {error && (
          <div className="text-sm text-destructive">{error}</div>
        )}
        <p className="text-xs text-muted-foreground">
          Enter your license key to activate Dagu Pro features. You can obtain a
          key from{' '}
          <a
            href="https://console.dagu.sh"
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:no-underline"
          >
            console.dagu.sh
          </a>
          .
        </p>
      </div>
    </div>
  );
}
