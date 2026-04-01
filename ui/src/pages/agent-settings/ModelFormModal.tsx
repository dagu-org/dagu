import { Loader2 } from 'lucide-react';
import { useState, useEffect, useContext, useRef } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { getAuthHeaders } from '@/lib/authHeaders';

type ModelConfig = components['schemas']['ModelConfigResponse'];
type ModelPreset = components['schemas']['ModelPreset'];
type DiscoveredProviderModel = components['schemas']['DiscoveredProviderModel'];
type DiscoverProviderMetadataResponse = components['schemas']['DiscoverProviderMetadataResponse'];

const LOCAL_DISCOVERY_DEBOUNCE_MS = 400;

// Mirrors internal/agent/store.go:GenerateSlugID
function generateSlugId(name: string): string {
  return name.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

const LLM_PROVIDERS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'local', label: 'Local' },
  { value: 'zai', label: 'Z.AI' },
];

interface ModelFormModalProps {
  open: boolean;
  model?: ModelConfig;
  presets: ModelPreset[];
  onClose: () => void;
  onSuccess: () => void;
}

export function ModelFormModal({ open, model, presets, onClose, onSuccess }: ModelFormModalProps) {
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);
  const isEditing = !!model;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [configId, setConfigId] = useState('');
  const [customId, setCustomId] = useState(false);
  const [name, setName] = useState('');
  const [provider, setProvider] = useState('anthropic');
  const [modelId, setModelId] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [description, setDescription] = useState('');
  const [contextWindow, setContextWindow] = useState<number | ''>('');
  const [maxOutputTokens, setMaxOutputTokens] = useState<number | ''>('');
  const [inputCostPer1M, setInputCostPer1M] = useState<number | ''>('');
  const [outputCostPer1M, setOutputCostPer1M] = useState<number | ''>('');
  const [supportsThinking, setSupportsThinking] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isDiscovering, setIsDiscovering] = useState(false);
  const [discoveryError, setDiscoveryError] = useState<string | null>(null);
  const [discoveryWarnings, setDiscoveryWarnings] = useState<string[]>([]);
  const [discoveredModels, setDiscoveredModels] = useState<DiscoveredProviderModel[]>([]);
  const [modelTouched, setModelTouched] = useState(false);

  const discoveryRequestIdRef = useRef(0);
  const discoveryAbortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!open) {
      resetForm();
      resetDiscoveryState(false);
      setError(null);
      return;
    }

    if (model) {
      setConfigId(model.id);
      setName(model.name);
      setProvider(model.provider);
      setModelId(model.model);
      setBaseUrl(model.baseUrl ?? '');
      setDescription(model.description ?? '');
      setContextWindow(model.contextWindow ?? '');
      setMaxOutputTokens(model.maxOutputTokens ?? '');
      setInputCostPer1M(model.inputCostPer1M ?? '');
      setOutputCostPer1M(model.outputCostPer1M ?? '');
      setSupportsThinking(model.supportsThinking ?? false);
      setApiKey('');
      resetDiscoveryState(true);
    } else {
      resetForm();
      resetDiscoveryState(false);
    }
    setError(null);
  }, [open, model]);

  useEffect(() => {
    if (!open || provider !== 'local') {
      cancelDiscoveryRequest();
      clearDiscoveryState();
      return;
    }

    const trimmedBaseUrl = baseUrl.trim();
    if (!trimmedBaseUrl || !isValidDiscoveryBaseUrl(trimmedBaseUrl)) {
      cancelDiscoveryRequest();
      clearDiscoveryState();
      return;
    }

    const controller = new AbortController();
    const requestId = discoveryRequestIdRef.current + 1;
    discoveryRequestIdRef.current = requestId;
    discoveryAbortRef.current?.abort();
    discoveryAbortRef.current = controller;

    setIsDiscovering(true);
    setDiscoveryError(null);
    setDiscoveryWarnings([]);
    setDiscoveredModels([]);

    const timeoutId = window.setTimeout(async () => {
      try {
        const response = await fetch(
          `${config.apiURL}/settings/agent/provider-metadata/discover?remoteNode=${encodeURIComponent(remoteNode)}`,
          {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({
              provider,
              baseUrl: trimmedBaseUrl,
              apiKey: apiKey || undefined,
            }),
            signal: controller.signal,
          }
        );

        const data = (await response.json().catch(() => ({}))) as
          | DiscoverProviderMetadataResponse
          | { message?: string };

        if (requestId !== discoveryRequestIdRef.current || controller.signal.aborted) {
          return;
        }

        if (!response.ok) {
          setDiscoveryError(data.message || 'Failed to discover local models');
          setDiscoveredModels([]);
          setDiscoveryWarnings([]);
          return;
        }

        const discoveryResponse = data as DiscoverProviderMetadataResponse;
        setDiscoveredModels(discoveryResponse.models || []);
        setDiscoveryWarnings(discoveryResponse.warnings || []);
        setDiscoveryError(discoveryResponse.error || null);
      } catch (err) {
        if (controller.signal.aborted || (err instanceof DOMException && err.name === 'AbortError')) {
          return;
        }
        if (requestId !== discoveryRequestIdRef.current) {
          return;
        }
        setDiscoveryError(err instanceof Error ? err.message : 'Failed to discover local models');
        setDiscoveredModels([]);
        setDiscoveryWarnings([]);
      } finally {
        if (requestId === discoveryRequestIdRef.current && !controller.signal.aborted) {
          setIsDiscovering(false);
        }
      }
    }, LOCAL_DISCOVERY_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timeoutId);
      controller.abort();
      if (discoveryAbortRef.current === controller) {
        discoveryAbortRef.current = null;
      }
    };
  }, [apiKey, baseUrl, config.apiURL, open, provider, remoteNode]);

  useEffect(() => {
    if (!open || provider !== 'local') {
      return;
    }
    if (modelTouched || modelId.trim() !== '' || discoveredModels.length !== 1) {
      return;
    }
    setModelId(discoveredModels[0].id);
  }, [discoveredModels, modelId, modelTouched, open, provider]);

  function resetForm() {
    setConfigId('');
    setCustomId(false);
    setName('');
    setProvider('anthropic');
    setModelId('');
    setApiKey('');
    setBaseUrl('');
    setDescription('');
    setContextWindow('');
    setMaxOutputTokens('');
    setInputCostPer1M('');
    setOutputCostPer1M('');
    setSupportsThinking(false);
  }

  function cancelDiscoveryRequest() {
    discoveryAbortRef.current?.abort();
    discoveryAbortRef.current = null;
    discoveryRequestIdRef.current += 1;
  }

  function clearDiscoveryState() {
    setIsDiscovering(false);
    setDiscoveryError(null);
    setDiscoveryWarnings([]);
    setDiscoveredModels([]);
  }

  function resetDiscoveryState(initialModelTouched: boolean) {
    cancelDiscoveryRequest();
    clearDiscoveryState();
    setModelTouched(initialModelTouched);
  }

  const handlePresetSelect = (presetName: string) => {
    const preset = presets.find((p) => p.name === presetName);
    if (preset) {
      setCustomId(false);
      setConfigId(generateSlugId(preset.name));
      setName(preset.name);
      setProvider(preset.provider);
      setModelId(preset.model);
      setDescription(preset.description ?? '');
      setContextWindow(preset.contextWindow ?? '');
      setMaxOutputTokens(preset.maxOutputTokens ?? '');
      setInputCostPer1M(preset.inputCostPer1M ?? '');
      setOutputCostPer1M(preset.outputCostPer1M ?? '');
      setSupportsThinking(preset.supportsThinking ?? false);
      setModelTouched(true);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      const body: Record<string, unknown> = {
        id: !isEditing && configId ? configId : undefined,
        name,
        provider,
        model: modelId,
        baseUrl: baseUrl || undefined,
        description: description || undefined,
        contextWindow: contextWindow !== '' ? contextWindow : undefined,
        maxOutputTokens: maxOutputTokens !== '' ? maxOutputTokens : undefined,
        inputCostPer1M: inputCostPer1M !== '' ? inputCostPer1M : undefined,
        outputCostPer1M: outputCostPer1M !== '' ? outputCostPer1M : undefined,
        supportsThinking,
      };

      if (apiKey) {
        body.apiKey = apiKey;
      }

      const url = isEditing
        ? `${config.apiURL}/settings/agent/models/${model.id}?remoteNode=${encodeURIComponent(remoteNode)}`
        : `${config.apiURL}/settings/agent/models?remoteNode=${encodeURIComponent(remoteNode)}`;

      const response = await fetch(url, {
        method: isEditing ? 'PATCH' : 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || `Failed to ${isEditing ? 'update' : 'create'} model`);
      }

      resetForm();
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setIsLoading(false);
    }
  };

  const hasValidDiscoveryBaseUrl = isValidDiscoveryBaseUrl(baseUrl.trim());
  const discoveredModelValue = discoveredModels.some((candidate) => candidate.id === modelId)
    ? modelId
    : undefined;
  const showNoDiscoveredModels =
    open &&
    provider === 'local' &&
    hasValidDiscoveryBaseUrl &&
    !isDiscovering &&
    !discoveryError &&
    discoveredModels.length === 0;
  const combinedDiscoveryWarning = discoveryWarnings.join(' ');

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          onClose();
        }
      }}
    >
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Model' : 'Add Model'}</DialogTitle>
          <DialogDescription>
            Configure a model provider and choose the model identifier used by the agent.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          <div className="space-y-3 py-3">
            {error && (
              <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
                {error}
              </div>
            )}

            {!isEditing && presets.length > 0 && (
              <div className="space-y-1.5">
                <Label className="text-sm">Import from Preset</Label>
                <Select onValueChange={handlePresetSelect}>
                  <SelectTrigger className="h-8">
                    <SelectValue placeholder="Select a preset..." />
                  </SelectTrigger>
                  <SelectContent>
                    {presets.map((p) => (
                      <SelectItem key={p.name} value={p.name}>
                        {p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="model-name" className="text-sm">Name</Label>
              <Input
                id="model-name"
                value={name}
                onChange={(e) => {
                  const v = e.target.value;
                  setName(v);
                  if (!isEditing && !customId) {
                    setConfigId(generateSlugId(v));
                  }
                }}
                placeholder="My Model"
                className="h-8"
                required
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="model-config-id" className="text-sm">ID {isEditing ? '' : '(optional)'}</Label>
              <Input
                id="model-config-id"
                value={configId}
                onChange={(e) => {
                  setConfigId(e.target.value);
                  setCustomId(true);
                }}
                placeholder="Auto-generated from name if empty"
                className="h-8"
                readOnly={isEditing}
                disabled={isEditing}
              />
              {!isEditing && (
                <p className="text-xs text-muted-foreground">
                  Lowercase letters, numbers, and hyphens only
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="model-provider" className="text-sm">Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger id="model-provider" className="h-8">
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
              <Label htmlFor="model-id" className="text-sm">Model</Label>
              <Input
                id="model-id"
                value={modelId}
                onChange={(e) => {
                  setModelTouched(true);
                  setModelId(e.target.value);
                }}
                placeholder="claude-sonnet-4-5"
                className="h-8"
                required
              />
              {provider === 'local' && discoveredModels.length > 0 && (
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Discovered Models</Label>
                  <Select
                    value={discoveredModelValue}
                    onValueChange={(value) => {
                      setModelTouched(true);
                      setModelId(value);
                    }}
                  >
                    <SelectTrigger className="h-8" aria-label="Discovered Models">
                      <SelectValue placeholder="Select a discovered model..." />
                    </SelectTrigger>
                    <SelectContent>
                      {discoveredModels.map((discoveredModel) => (
                        <SelectItem key={discoveredModel.id} value={discoveredModel.id}>
                          {discoveredModel.displayName || discoveredModel.id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-xs text-muted-foreground">
                    Selecting a discovered model fills the Model field. Manual entry still works.
                  </p>
                </div>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="model-api-key" className="text-sm">API Key</Label>
              <Input
                id="model-api-key"
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={isEditing && model?.apiKeyConfigured ? '********' : 'Enter API key'}
                className="h-8"
              />
              {isEditing && model?.apiKeyConfigured && (
                <p className="text-xs text-muted-foreground">
                  Leave empty to keep existing key
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="model-base-url" className="text-sm">Base URL (optional)</Label>
              <Input
                id="model-base-url"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="Custom API endpoint"
                className="h-8"
              />
              {provider === 'local' && isDiscovering && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground" role="status">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  Discovering local models...
                </div>
              )}
              {provider === 'local' && !isDiscovering && discoveryError && (
                <p className="text-xs text-destructive">{discoveryError}</p>
              )}
              {provider === 'local' && !isDiscovering && !discoveryError && combinedDiscoveryWarning && (
                <p className="text-xs text-muted-foreground">{combinedDiscoveryWarning}</p>
              )}
              {provider === 'local' && showNoDiscoveredModels && (
                <p className="text-xs text-muted-foreground">
                  No models were discovered for this Base URL. You can still enter a model manually.
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="model-description" className="text-sm">Description (optional)</Label>
              <Input
                id="model-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Description"
                className="h-8"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="model-context-window" className="text-sm">Context Window (optional)</Label>
                <Input
                  id="model-context-window"
                  type="number"
                  min={0}
                  value={contextWindow}
                  onChange={(e) => setContextWindow(e.target.value ? Number(e.target.value) : '')}
                  placeholder="e.g. 200000"
                  className="h-8"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="model-max-output" className="text-sm">Max Output Tokens (optional)</Label>
                <Input
                  id="model-max-output"
                  type="number"
                  min={0}
                  value={maxOutputTokens}
                  onChange={(e) => setMaxOutputTokens(e.target.value ? Number(e.target.value) : '')}
                  placeholder="e.g. 128000"
                  className="h-8"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="model-input-cost" className="text-sm">Input Cost / 1M tokens (optional)</Label>
                <Input
                  id="model-input-cost"
                  type="number"
                  min={0}
                  step="0.01"
                  value={inputCostPer1M}
                  onChange={(e) => setInputCostPer1M(e.target.value ? Number(e.target.value) : '')}
                  placeholder="e.g. 3.00"
                  className="h-8"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="model-output-cost" className="text-sm">Output Cost / 1M tokens (optional)</Label>
                <Input
                  id="model-output-cost"
                  type="number"
                  min={0}
                  step="0.01"
                  value={outputCostPer1M}
                  onChange={(e) => setOutputCostPer1M(e.target.value ? Number(e.target.value) : '')}
                  placeholder="e.g. 15.00"
                  className="h-8"
                />
              </div>
            </div>

            <div className="flex items-center justify-between">
              <Label htmlFor="model-thinking" className="text-sm">Supports Thinking (optional)</Label>
              <Switch
                id="model-thinking"
                checked={supportsThinking}
                onCheckedChange={setSupportsThinking}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} size="sm" className="h-8">
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading || !name || !modelId} size="sm" className="h-8">
              {isLoading ? 'Saving...' : isEditing ? 'Save Changes' : 'Add Model'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function isValidDiscoveryBaseUrl(value: string): boolean {
  if (!value) {
    return false;
  }

  try {
    const parsed = new URL(value);
    return Boolean(parsed.host) && (parsed.protocol === 'http:' || parsed.protocol === 'https:');
  } catch {
    return false;
  }
}
