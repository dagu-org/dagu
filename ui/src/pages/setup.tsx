import { useState, useEffect, useCallback, useContext } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useConfig, useUpdateConfig } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { components, CreateModelConfigRequestProvider } from '@/api/v1/schema';
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
import {
  AlertCircle,
  UserPlus,
  Bot,
  ChevronRight,
  SkipForward,
  Loader2,
} from 'lucide-react';

type ModelPreset = components['schemas']['ModelPreset'];

const PROVIDERS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
] as const;

function generateSlugId(name: string): string {
  const slug = name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
  return slug || 'model';
}

export default function SetupPage() {
  const config = useConfig();
  const updateConfig = useUpdateConfig();
  const { setupRequired, setup, completeSetup } = useAuth();
  const navigate = useNavigate();

  const [currentStep, setCurrentStep] = useState<1 | 2>(1);
  const [setupDone, setSetupDone] = useState(false);

  // Step 1 state
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [step1Error, setStep1Error] = useState<string | null>(null);
  const [step1Loading, setStep1Loading] = useState(false);

  // Step 2 state
  const [agentEnabled, setAgentEnabled] = useState(true);
  const [selectedProvider, setSelectedProvider] = useState('anthropic');
  const [selectedModel, setSelectedModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [step2Error, setStep2Error] = useState<string | null>(null);
  const [step2Loading, setStep2Loading] = useState(false);
  const [presets, setPresets] = useState<ModelPreset[]>([]);
  const [presetsLoading, setPresetsLoading] = useState(false);

  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Redirect away if setup is already complete (e.g., user navigated here directly).
  // Skip if we just completed setup ourselves — we handle navigation manually.
  useEffect(() => {
    if (!setupRequired && !setupDone) {
      navigate('/login', { replace: true });
    }
  }, [setupRequired, setupDone, navigate]);

  // Fetch presets when entering step 2
  const fetchPresets = useCallback(async () => {
    setPresetsLoading(true);
    try {
      const { data } = await client.GET('/settings/agent/model-presets', {
        params: { query: { remoteNode } },
      });
      if (data) {
        setPresets(data.presets || []);
      }
    } catch {
      // Presets are optional, don't block the user
    } finally {
      setPresetsLoading(false);
    }
  }, [client, remoteNode]);

  useEffect(() => {
    if (currentStep === 2) {
      fetchPresets();
    }
  }, [currentStep, fetchPresets]);

  // Step 1: Create admin account
  const handleStep1Submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStep1Error(null);

    const trimmedUsername = username.trim();
    if (trimmedUsername.length < 3) {
      setStep1Error('Username must be at least 3 characters');
      return;
    }

    if (password.length < 8) {
      setStep1Error('Password must be at least 8 characters');
      return;
    }

    if (password !== confirmPassword) {
      setStep1Error('Passwords do not match');
      return;
    }

    setStep1Loading(true);

    try {
      const result = await setup(trimmedUsername, password);
      setSetupDone(true);
      completeSetup(result);
      setCurrentStep(2);
    } catch (err) {
      if ((err as any)?.status === 403) {
        navigate('/login', { replace: true });
        return;
      }
      setStep1Error(err instanceof Error ? err.message : 'Setup failed');
    } finally {
      setStep1Loading(false);
    }
  };

  // Step 2: Configure agent
  const handleStep2Submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStep2Error(null);

    if (!agentEnabled) {
      await client.PATCH('/settings/agent', {
        params: { query: { remoteNode } },
        body: { enabled: false },
      });
      navigate('/', { replace: true });
      return;
    }

    if (!selectedModel) {
      setStep2Error('Please select a model');
      return;
    }

    if (!apiKey.trim()) {
      setStep2Error('Please enter an API key');
      return;
    }

    setStep2Loading(true);

    try {
      // 1. Enable agent
      const { error: enableError } = await client.PATCH('/settings/agent', {
        params: { query: { remoteNode } },
        body: { enabled: true },
      });
      if (enableError) {
        throw new Error(enableError.message || 'Failed to enable agent');
      }

      // 2. Find the selected preset to populate model details
      const preset = presets.find((p) => p.name === selectedModel);
      if (!preset) {
        throw new Error('Selected model not found');
      }

      // 3. Create model config
      const supportedProviders = [
        CreateModelConfigRequestProvider.anthropic,
        CreateModelConfigRequestProvider.openai,
        CreateModelConfigRequestProvider.gemini,
      ] as const;
      const isValidProvider = (v: string): v is CreateModelConfigRequestProvider =>
        (supportedProviders as readonly string[]).includes(v);
      if (!isValidProvider(preset.provider)) {
        throw new Error(`Unsupported provider: ${preset.provider}`);
      }
      const { data: createdModel, error: createError } = await client.POST(
        '/settings/agent/models',
        {
          params: { query: { remoteNode } },
          body: {
            id: generateSlugId(preset.name),
            name: preset.name,
            provider: preset.provider,
            model: preset.model,
            apiKey: apiKey.trim(),
            description: preset.description || undefined,
            contextWindow: preset.contextWindow || undefined,
            maxOutputTokens: preset.maxOutputTokens || undefined,
            inputCostPer1M: preset.inputCostPer1M || undefined,
            outputCostPer1M: preset.outputCostPer1M || undefined,
            supportsThinking: preset.supportsThinking || false,
          },
        }
      );
      if (createError) {
        throw new Error(createError.message || 'Failed to create model');
      }

      // 4. Set as default model
      const modelId = createdModel?.id || generateSlugId(preset.name);
      const { error: defaultError } = await client.PUT(
        '/settings/agent/default-model',
        {
          params: { query: { remoteNode } },
          body: { modelId },
        }
      );
      if (defaultError) {
        throw new Error(
          defaultError.message || 'Failed to set default model'
        );
      }

      updateConfig({ agentEnabled: true });
      navigate('/', { replace: true, state: { openAgent: true } });
    } catch (err) {
      setStep2Error(err instanceof Error ? err.message : 'Configuration failed');
    } finally {
      setStep2Loading(false);
    }
  };

  const handleSkip = () => {
    navigate('/', { replace: true });
  };

  // Filtered presets by selected provider
  const filteredPresets = presets.filter(
    (p) => p.provider === selectedProvider
  );

  // Reset model selection when provider changes
  useEffect(() => {
    setSelectedModel('');
  }, [selectedProvider]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted/50">
      <div className="w-full max-w-sm p-6 space-y-6">
        {/* Header */}
        <div className="text-center space-y-2">
          <h1 className="text-2xl font-bold">{config.title || 'Dagu'}</h1>
          <p className="text-sm text-muted-foreground">
            {currentStep === 1
              ? 'Create your admin account to get started'
              : 'Configure the AI agent (optional)'}
          </p>
        </div>

        {/* Step indicator */}
        <div className="flex items-center justify-center gap-2">
          <div
            className={`h-1.5 w-8 rounded-full transition-colors ${
              currentStep === 1
                ? 'bg-primary'
                : 'bg-primary/30'
            }`}
          />
          <div
            className={`h-1.5 w-8 rounded-full transition-colors ${
              currentStep === 2
                ? 'bg-primary'
                : 'bg-muted-foreground/20'
            }`}
          />
          <span className="text-xs text-muted-foreground ml-2">
            Step {currentStep} of 2
          </span>
        </div>

        {/* Step 1: Create Admin Account */}
        {currentStep === 1 && (
          <div className="space-y-4">
            {step1Error && (
              <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                <span>{step1Error}</span>
              </div>
            )}

            <form onSubmit={handleStep1Submit} className="space-y-4">
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
                  autoComplete="username"
                  autoFocus
                  className="h-9"
                />
              </div>

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
                />
                <p className="text-xs text-muted-foreground">
                  Minimum 8 characters
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="confirmPassword" className="text-sm">
                  Confirm Password
                </Label>
                <Input
                  id="confirmPassword"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  className="h-9"
                />
              </div>

              <Button
                type="submit"
                className="w-full h-9"
                disabled={step1Loading}
              >
                {step1Loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Creating account...
                  </>
                ) : (
                  <>
                    <UserPlus className="h-4 w-4" />
                    Continue
                    <ChevronRight className="h-4 w-4" />
                  </>
                )}
              </Button>
            </form>
          </div>
        )}

        {/* Step 2: Configure AI Agent */}
        {currentStep === 2 && (
          <div className="space-y-4">
            {step2Error && (
              <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                <span>{step2Error}</span>
              </div>
            )}

            <form onSubmit={handleStep2Submit} className="space-y-4" autoComplete="off">
              {/* Enable toggle */}
              <div className="flex items-center justify-between rounded-md border border-border/60 p-3">
                <div className="flex items-center gap-2">
                  <Bot className="h-4 w-4 text-muted-foreground" />
                  <div>
                    <Label htmlFor="agent-toggle" className="text-sm font-medium">
                      Enable AI Agent
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      AI-powered workflow generation
                    </p>
                  </div>
                </div>
                <Switch
                  id="agent-toggle"
                  checked={agentEnabled}
                  onCheckedChange={setAgentEnabled}
                />
              </div>

              {/* Model configuration (shown when enabled) */}
              {agentEnabled && (
                <div className="space-y-3">
                  {presetsLoading ? (
                    <div className="flex items-center justify-center py-4">
                      <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                      <span className="text-sm text-muted-foreground ml-2">
                        Loading models...
                      </span>
                    </div>
                  ) : (
                    <>
                      {/* Provider selection */}
                      <div className="space-y-1.5">
                        <Label className="text-sm">Provider</Label>
                        <div className="grid grid-cols-3 gap-2">
                          {PROVIDERS.map((p) => (
                            <button
                              key={p.value}
                              type="button"
                              onClick={() => setSelectedProvider(p.value)}
                              className={`rounded-md border px-3 py-2 text-xs font-medium transition-colors ${
                                selectedProvider === p.value
                                  ? 'border-primary bg-primary/5 text-primary'
                                  : 'border-border/60 text-muted-foreground hover:border-border hover:text-foreground'
                              }`}
                            >
                              {p.label}
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Model selection */}
                      <div className="space-y-1.5">
                        <Label htmlFor="model-select" className="text-sm">
                          Model
                        </Label>
                        {filteredPresets.length > 0 ? (
                          <Select
                            value={selectedModel}
                            onValueChange={setSelectedModel}
                          >
                            <SelectTrigger id="model-select" className="h-9">
                              <SelectValue placeholder="Select a model..." />
                            </SelectTrigger>
                            <SelectContent>
                              {filteredPresets.map((p) => (
                                <SelectItem key={p.name} value={p.name}>
                                  <span>{p.name}</span>
                                  {p.description && (
                                    <span className="text-muted-foreground ml-1">
                                      — {p.description}
                                    </span>
                                  )}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : (
                          <p className="text-xs text-muted-foreground py-2">
                            No presets available for this provider.
                          </p>
                        )}
                      </div>

                      {/* API Key */}
                      <div className="space-y-1.5">
                        <Label htmlFor="api-key" className="text-sm">
                          API Key
                        </Label>
                        <Input
                          id="api-key"
                          type="password"
                          value={apiKey}
                          onChange={(e) => setApiKey(e.target.value)}
                          placeholder="Enter your API key"
                          autoComplete="off"
                          className="h-9"
                        />
                      </div>
                    </>
                  )}
                </div>
              )}

              {/* Action buttons */}
              <div className="space-y-2">
                <Button
                  type="submit"
                  className="w-full h-9"
                  disabled={step2Loading || (agentEnabled && presetsLoading)}
                >
                  {step2Loading ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Setting up...
                    </>
                  ) : agentEnabled ? (
                    <>
                      <Bot className="h-4 w-4" />
                      Complete Setup
                    </>
                  ) : (
                    <>
                      <ChevronRight className="h-4 w-4" />
                      Complete Setup
                    </>
                  )}
                </Button>

                <Button
                  type="button"
                  variant="ghost"
                  className="w-full h-9 text-muted-foreground"
                  onClick={handleSkip}
                  disabled={step2Loading}
                >
                  <SkipForward className="h-4 w-4" />
                  Skip for now
                </Button>
              </div>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
