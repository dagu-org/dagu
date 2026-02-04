import React, { useContext, useEffect, useState } from 'react';
import { Bot, Loader2, Save } from 'lucide-react';
import { components } from '@/api/v1/schema';
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
import { useIsAdmin } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { getAuthHeaders } from '@/lib/authHeaders';

type AgentConfig = components['schemas']['AgentConfigResponse'];

const LLM_PROVIDERS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'local', label: 'Local' },
];

export default function AgentSettingsPage(): React.ReactNode {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [enabled, setEnabled] = useState(false);
  const [provider, setProvider] = useState('anthropic');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [apiKeyConfigured, setApiKeyConfigured] = useState(false);
  const [baseUrl, setBaseUrl] = useState('');

  const remoteNode = encodeURIComponent(
    appBarContext.selectedRemoteNode || 'local'
  );
  const agentSettingsUrl = `${config.apiURL}/settings/agent?remoteNode=${remoteNode}`;

  useEffect(() => {
    appBarContext.setTitle('Agent Settings');
  }, [appBarContext]);

  function updateFormState(data: AgentConfig): void {
    setEnabled(data.enabled ?? false);
    if (data.llm) {
      setProvider(data.llm.provider ?? 'anthropic');
      setModel(data.llm.model ?? '');
      setApiKeyConfigured(data.llm.apiKeyConfigured ?? false);
      setBaseUrl(data.llm.baseUrl ?? '');
    }
  }

  useEffect(() => {
    async function fetchConfig(): Promise<void> {
      try {
        const response = await fetch(agentSettingsUrl, {
          headers: getAuthHeaders(),
        });

        if (!response.ok) {
          throw new Error('Failed to fetch agent configuration');
        }

        const data: AgentConfig = await response.json();
        updateFormState(data);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to load configuration'
        );
      } finally {
        setIsLoading(false);
      }
    }

    fetchConfig();
  }, [agentSettingsUrl]);

  async function handleSave(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);

    const llmConfig: Record<string, string | undefined> = {
      provider,
      model,
      baseUrl: baseUrl || undefined,
    };

    if (apiKey) {
      llmConfig.apiKey = apiKey;
    }

    try {
      const response = await fetch(agentSettingsUrl, {
        method: 'PATCH',
        headers: getAuthHeaders(),
        body: JSON.stringify({ enabled, llm: llmConfig }),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to save configuration');
      }

      const data: AgentConfig = await response.json();
      updateFormState(data);
      setApiKey('');
      setSuccess('Configuration saved successfully');

      setTimeout(() => window.location.reload(), 500);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to save configuration'
      );
    } finally {
      setIsSaving(false);
    }
  }

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
    <div className="space-y-4 max-w-7xl">
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
          <div className="border-t pt-4 space-y-4">
            <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Bot className="h-4 w-4" />
              LLM Configuration
            </div>

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

            <div className="space-y-1.5">
              <Label htmlFor="model" className="text-sm">
                Model
              </Label>
              <Input
                id="model"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="claude-sonnet-4-5"
                className="h-8"
              />
              <p className="text-xs text-muted-foreground">
                The model ID to use for completions
              </p>
            </div>

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
        )}

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
