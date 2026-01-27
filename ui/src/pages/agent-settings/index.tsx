import { components } from '@/api/v2/schema';
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
import { AppBarContext } from '@/contexts/AppBarContext';
import { TOKEN_KEY, useIsAdmin } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { Bot, Loader2, Save } from 'lucide-react';
import { useCallback, useContext, useEffect, useState } from 'react';

type AgentConfig = components['schemas']['AgentConfigResponse'];
type AgentLLMConfig = components['schemas']['AgentLLMConfig'];

const LLM_PROVIDERS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'local', label: 'Local' },
];

export default function AgentSettingsPage() {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Form state
  const [enabled, setEnabled] = useState(false);
  const [provider, setProvider] = useState('anthropic');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [apiKeyConfigured, setApiKeyConfigured] = useState(false);
  const [baseUrl, setBaseUrl] = useState('');

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('Agent Settings');
  }, [appBarContext]);

  const fetchConfig = useCallback(async () => {
    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = appBarContext.selectedRemoteNode || 'local';
      const response = await fetch(
        `${config.apiURL}/agent/config?remoteNode=${remoteNode}`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!response.ok) {
        throw new Error('Failed to fetch agent configuration');
      }

      const data: AgentConfig = await response.json();
      setEnabled(data.enabled || false);
      if (data.llm) {
        setProvider(data.llm.provider || 'anthropic');
        setModel(data.llm.model || '');
        setApiKeyConfigured(data.llm.apiKeyConfigured || false);
        setBaseUrl(data.llm.baseUrl || '');
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to load configuration'
      );
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL, appBarContext.selectedRemoteNode]);

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);
    setSuccess(null);

    try {
      const token = localStorage.getItem(TOKEN_KEY);
      const remoteNode = appBarContext.selectedRemoteNode || 'local';

      const body: {
        enabled?: boolean;
        llm?: {
          provider?: string;
          model?: string;
          apiKey?: string;
          baseUrl?: string;
        };
      } = {
        enabled,
        llm: {
          provider,
          model,
          baseUrl: baseUrl || undefined,
        },
      };

      // Only include apiKey if it was changed
      if (apiKey) {
        body.llm!.apiKey = apiKey;
      }

      const response = await fetch(
        `${config.apiURL}/agent/config?remoteNode=${remoteNode}`,
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
        throw new Error(data.message || 'Failed to save configuration');
      }

      const data: AgentConfig = await response.json();
      setEnabled(data.enabled || false);
      if (data.llm) {
        setProvider(data.llm.provider || 'anthropic');
        setModel(data.llm.model || '');
        setApiKeyConfigured(data.llm.apiKeyConfigured || false);
        setBaseUrl(data.llm.baseUrl || '');
      }
      setApiKey(''); // Clear the API key field after save
      setSuccess('Configuration saved successfully');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to save configuration'
      );
    } finally {
      setIsSaving(false);
    }
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">
          You do not have permission to access this page.
        </p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-lg font-semibold">Agent Settings</h1>
        <p className="text-sm text-muted-foreground">
          Configure the AI assistant for workflow generation
        </p>
      </div>

      {error && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          {error}
        </div>
      )}

      {success && (
        <div className="p-3 text-sm text-green-600 bg-green-500/10 rounded-md">
          {success}
        </div>
      )}

      <div className="card-obsidian p-4 space-y-6 max-w-xl">
        {/* Enable/Disable Toggle */}
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <Label htmlFor="enabled" className="text-sm font-medium">
              Enable Agent
            </Label>
            <p className="text-xs text-muted-foreground">
              Turn on the AI assistant feature
            </p>
          </div>
          <Switch
            id="enabled"
            checked={enabled}
            onCheckedChange={setEnabled}
          />
        </div>

        {enabled && (
          <>
            <div className="border-t pt-4 space-y-4">
              <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Bot className="h-4 w-4" />
                LLM Configuration
              </div>

              {/* Provider Selection */}
              <div className="space-y-1.5">
                <Label htmlFor="provider" className="text-sm">
                  Provider
                </Label>
                <Select value={provider} onValueChange={setProvider}>
                  <SelectTrigger id="provider" className="h-8">
                    <SelectValue placeholder="Select provider" />
                  </SelectTrigger>
                  <SelectContent>
                    {LLM_PROVIDERS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Model Input */}
              <div className="space-y-1.5">
                <Label htmlFor="model" className="text-sm">
                  Model
                </Label>
                <Input
                  id="model"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  placeholder="claude-sonnet-4-20250514"
                  className="h-8"
                />
                <p className="text-xs text-muted-foreground">
                  The model ID to use for completions
                </p>
              </div>

              {/* API Key Input */}
              <div className="space-y-1.5">
                <Label htmlFor="apiKey" className="text-sm">
                  API Key
                </Label>
                <Input
                  id="apiKey"
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={apiKeyConfigured ? '********' : 'Enter API key'}
                  className="h-8"
                />
                <p className="text-xs text-muted-foreground">
                  {apiKeyConfigured
                    ? 'An API key is configured. Leave empty to keep it unchanged.'
                    : 'Required for external LLM providers'}
                </p>
              </div>

              {/* Base URL Input */}
              <div className="space-y-1.5">
                <Label htmlFor="baseUrl" className="text-sm">
                  Base URL (optional)
                </Label>
                <Input
                  id="baseUrl"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="Custom API endpoint"
                  className="h-8"
                />
                <p className="text-xs text-muted-foreground">
                  Override the default API endpoint
                </p>
              </div>
            </div>
          </>
        )}

        {/* Save Button */}
        <div className="pt-2">
          <Button
            onClick={handleSave}
            disabled={isSaving}
            size="sm"
            className="h-8"
          >
            {isSaving ? (
              <>
                <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                Saving...
              </>
            ) : (
              <>
                <Save className="h-4 w-4 mr-1.5" />
                Save Settings
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}
