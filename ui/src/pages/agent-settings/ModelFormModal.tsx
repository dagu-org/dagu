import { useState, useEffect, useContext } from 'react';
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { getAuthHeaders } from '@/lib/authHeaders';

type ModelConfig = components['schemas']['ModelConfigResponse'];
type ModelPreset = components['schemas']['ModelPreset'];

const LLM_PROVIDERS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'local', label: 'Local' },
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

  // Only populate fields when opening in edit mode.
  // In create mode, preserve values so accidental close doesn't lose input.
  useEffect(() => {
    if (open && model) {
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
    }
    if (open) {
      setError(null);
    }
  }, [open, model]);

  function resetForm() {
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

  const handlePresetSelect = (presetName: string) => {
    const preset = presets.find((p) => p.name === presetName);
    if (preset) {
      setName(preset.name);
      setProvider(preset.provider);
      setModelId(preset.model);
      setDescription(preset.description ?? '');
      setContextWindow(preset.contextWindow ?? '');
      setMaxOutputTokens(preset.maxOutputTokens ?? '');
      setInputCostPer1M(preset.inputCostPer1M ?? '');
      setOutputCostPer1M(preset.outputCostPer1M ?? '');
      setSupportsThinking(preset.supportsThinking ?? false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');

      const body: Record<string, unknown> = {
        name,
        provider,
        model: modelId,
        baseUrl: baseUrl || undefined,
        description: description || undefined,
        contextWindow: contextWindow !== '' ? contextWindow : undefined,
        maxOutputTokens: maxOutputTokens !== '' ? maxOutputTokens : undefined,
        inputCostPer1M: inputCostPer1M !== '' ? inputCostPer1M : undefined,
        outputCostPer1M: outputCostPer1M !== '' ? outputCostPer1M : undefined,
        supportsThinking: supportsThinking || undefined,
      };

      if (apiKey) {
        body.apiKey = apiKey;
      }

      const url = isEditing
        ? `${config.apiURL}/settings/agent/models/${model.id}?remoteNode=${remoteNode}`
        : `${config.apiURL}/settings/agent/models?remoteNode=${remoteNode}`;

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

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Model' : 'Add Model'}</DialogTitle>
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
                onChange={(e) => setName(e.target.value)}
                placeholder="My Model"
                className="h-8"
                required
              />
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
                onChange={(e) => setModelId(e.target.value)}
                placeholder="claude-sonnet-4-5"
                className="h-8"
                required
              />
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
