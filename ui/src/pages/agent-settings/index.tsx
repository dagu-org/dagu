import { type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Bot, Loader2, MoreHorizontal, Pencil, Plus, Save, Shield, Star, Trash2 } from 'lucide-react';
import {
  AgentBashPolicyDefaultBehavior,
  AgentBashPolicyDenyBehavior,
  AgentBashRuleAction,
  components,
} from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';
import ConfirmModal from '@/ui/ConfirmModal';
import { ModelFormModal } from './ModelFormModal';

type ModelConfig = components['schemas']['ModelConfigResponse'];
type ModelPreset = components['schemas']['ModelPreset'];
type AgentToolPolicy = components['schemas']['AgentToolPolicy'];
type AgentBashRule = components['schemas']['AgentBashRule'];
type UpdateAgentConfigRequest = components['schemas']['UpdateAgentConfigRequest'];

type SoulOption = {
  id: string;
  name: string;
};

type SavedAgentConfig = {
  enabled: boolean;
  defaultModelId?: string;
  selectedSoulId?: string;
  toolPolicy: AgentToolPolicy;
  webSearchEnabled: boolean;
  webSearchMaxUses?: number;
};

type ToolMeta = {
  name: string;
  label: string;
  description: string;
};

function createDefaultToolPolicy(tools: ToolMeta[]): AgentToolPolicy {
  const toggles: Record<string, boolean> = {};
  for (const t of tools) {
    toggles[t.name] = true;
  }
  return {
    tools: toggles,
    bash: {
      rules: [],
      defaultBehavior: AgentBashPolicyDefaultBehavior.allow,
      denyBehavior: AgentBashPolicyDenyBehavior.ask_user,
    },
  };
}

function normalizeToolPolicy(policy: AgentToolPolicy | undefined, tools: ToolMeta[]): AgentToolPolicy {
  const defaults = createDefaultToolPolicy(tools);
  const merged = { ...defaults.tools, ...(policy?.tools || {}) };
  const bash = {
    rules: policy?.bash?.rules || defaults.bash?.rules || [],
    defaultBehavior: policy?.bash?.defaultBehavior || defaults.bash?.defaultBehavior,
    denyBehavior: policy?.bash?.denyBehavior || defaults.bash?.denyBehavior,
  };
  return { tools: merged, bash };
}

function canonicalizeToolPolicy(policy: AgentToolPolicy | undefined, tools: ToolMeta[]): AgentToolPolicy {
  const normalized = normalizeToolPolicy(policy, tools);
  const sortedToolsEntries = Object.entries(normalized.tools || {}).sort(([a], [b]) =>
    a.localeCompare(b)
  );
  const sortedTools = Object.fromEntries(sortedToolsEntries);
  const rules = (normalized.bash?.rules || []).map((rule) => ({
    ...rule,
    name: rule.name || '',
    enabled: rule.enabled ?? true,
  }));

  return {
    tools: sortedTools,
    bash: {
      rules,
      defaultBehavior: normalized.bash?.defaultBehavior || AgentBashPolicyDefaultBehavior.allow,
      denyBehavior: normalized.bash?.denyBehavior || AgentBashPolicyDenyBehavior.ask_user,
    },
  };
}

export default function AgentSettingsPage(): ReactNode {
  const client = useClient();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const [toolMetas, setToolMetas] = useState<ToolMeta[]>([]);

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [enabled, setEnabled] = useState(false);
  const [defaultModelId, setDefaultModelId] = useState<string | undefined>();
  const [toolPolicy, setToolPolicy] = useState<AgentToolPolicy>(() => createDefaultToolPolicy([]));
  const [savedConfig, setSavedConfig] = useState<SavedAgentConfig | null>(null);
  const [bashRuleIds, setBashRuleIds] = useState<string[]>([]);
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [presets, setPresets] = useState<ModelPreset[]>([]);
  const [souls, setSouls] = useState<SoulOption[]>([]);
  const [selectedSoulId, setSelectedSoulId] = useState<string | undefined>();
  const [webSearchEnabled, setWebSearchEnabled] = useState(false);
  const [webSearchMaxUses, setWebSearchMaxUses] = useState<number | undefined>();

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingModel, setEditingModel] = useState<ModelConfig | null>(null);
  const [deletingModel, setDeletingModel] = useState<ModelConfig | null>(null);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const bashRuleIdCounter = useRef(0);

  const nextBashRuleId = useCallback((): string => {
    bashRuleIdCounter.current += 1;
    return `bash_rule_${bashRuleIdCounter.current}`;
  }, []);

  const buildBashRuleIDs = useCallback((count: number): string[] => {
    return Array.from({ length: count }, () => nextBashRuleId());
  }, [nextBashRuleId]);

  useEffect(() => {
    appBarContext.setTitle('Agent Settings');
  }, [appBarContext]);

  const fetchTools = useCallback(async (): Promise<ToolMeta[]> => {
    try {
      const { data } = await client.GET('/settings/agent/tools', {
        params: { query: { remoteNode } },
      });
      const tools: ToolMeta[] = data?.tools || [];
      setToolMetas(tools);
      return tools;
    } catch (err) {
      console.error('Failed to load tool definitions', err);
      return [];
    }
  }, [client, remoteNode]);

  const fetchConfig = useCallback(async (tools: ToolMeta[]) => {
    try {
      const { data, error: apiError } = await client.GET('/settings/agent', {
        params: { query: { remoteNode } },
      });
      if (apiError) throw new Error('Failed to fetch agent configuration');
      const normalizedPolicy = normalizeToolPolicy(data.toolPolicy, tools);
      setEnabled(data.enabled ?? false);
      setDefaultModelId(data.defaultModelId);
      setSelectedSoulId(data.selectedSoulId ?? undefined);
      setToolPolicy(normalizedPolicy);
      setWebSearchEnabled(data.webSearch?.enabled ?? false);
      setWebSearchMaxUses(data.webSearch?.maxUses ?? undefined);
      setSavedConfig({
        enabled: data.enabled ?? false,
        defaultModelId: data.defaultModelId,
        selectedSoulId: data.selectedSoulId ?? undefined,
        toolPolicy: normalizedPolicy,
        webSearchEnabled: data.webSearch?.enabled ?? false,
        webSearchMaxUses: data.webSearch?.maxUses ?? undefined,
      });
      setBashRuleIds(buildBashRuleIDs(normalizedPolicy.bash?.rules?.length || 0));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load configuration');
    }
  }, [buildBashRuleIDs, client, remoteNode]);

  const fetchModels = useCallback(async () => {
    try {
      const { data, error: apiError } = await client.GET('/settings/agent/models', {
        params: { query: { remoteNode } },
      });
      if (apiError) throw new Error('Failed to fetch models');
      setModels(data.models || []);
      setDefaultModelId(data.defaultModelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load models');
    }
  }, [client, remoteNode]);

  const fetchPresets = useCallback(async () => {
    try {
      const { data } = await client.GET('/settings/agent/model-presets', {
        params: { query: { remoteNode } },
      });
      if (data) {
        setPresets(data.presets || []);
      }
    } catch {
      // Presets are optional, don't show error
    }
  }, [client, remoteNode]);

  const fetchSouls = useCallback(async () => {
    try {
      const { data } = await client.GET('/settings/agent/souls', {
        params: { query: { remoteNode } },
      });
      if (data) {
        setSouls((data.souls || []).map((s) => ({ id: s.id, name: s.name })));
      }
    } catch {
      // Souls fetch is best-effort
    }
  }, [client, remoteNode]);

  useEffect(() => {
    async function load() {
      const tools = await fetchTools();
      await Promise.all([fetchConfig(tools), fetchModels(), fetchPresets(), fetchSouls()]);
      setIsLoading(false);
    }
    load();
  }, [fetchTools, fetchConfig, fetchModels, fetchPresets, fetchSouls]);

  async function handleSaveConfig(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);

    try {
      const requestBody: UpdateAgentConfigRequest = {};
      const currentPolicyCanonical = canonicalizeToolPolicy(toolPolicy, toolMetas);
      const savedPolicyCanonical = canonicalizeToolPolicy(savedConfig?.toolPolicy, toolMetas);

      if (!savedConfig || enabled !== savedConfig.enabled) {
        requestBody.enabled = enabled;
      }
      if (!savedConfig || defaultModelId !== savedConfig.defaultModelId) {
        requestBody.defaultModelId = defaultModelId;
      }
      if (!savedConfig || selectedSoulId !== savedConfig.selectedSoulId) {
        requestBody.selectedSoulId = selectedSoulId;
      }
      if (!savedConfig || JSON.stringify(currentPolicyCanonical) !== JSON.stringify(savedPolicyCanonical)) {
        requestBody.toolPolicy = currentPolicyCanonical;
      }
      if (!savedConfig || webSearchEnabled !== savedConfig.webSearchEnabled || webSearchMaxUses !== savedConfig.webSearchMaxUses) {
        requestBody.webSearch = {
          enabled: webSearchEnabled,
          maxUses: webSearchMaxUses,
        };
      }

      if (Object.keys(requestBody).length === 0) {
        setSuccess('No changes to save');
        return;
      }

      const { data, error: apiError } = await client.PATCH('/settings/agent', {
        params: { query: { remoteNode } },
        body: requestBody,
      });

      if (apiError) {
        throw new Error(apiError.message || 'Failed to save configuration');
      }

      const normalizedPolicy = normalizeToolPolicy(data.toolPolicy, toolMetas);
      setEnabled(data.enabled ?? false);
      setDefaultModelId(data.defaultModelId);
      setSelectedSoulId(data.selectedSoulId ?? undefined);
      setToolPolicy(normalizedPolicy);
      setWebSearchEnabled(data.webSearch?.enabled ?? false);
      setWebSearchMaxUses(data.webSearch?.maxUses ?? undefined);
      setSavedConfig({
        enabled: data.enabled ?? false,
        defaultModelId: data.defaultModelId,
        selectedSoulId: data.selectedSoulId ?? undefined,
        toolPolicy: normalizedPolicy,
        webSearchEnabled: data.webSearch?.enabled ?? false,
        webSearchMaxUses: data.webSearch?.maxUses ?? undefined,
      });
      setSuccess('Configuration saved successfully');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save configuration');
    } finally {
      setIsSaving(false);
    }
  }

  async function handleSetDefault(modelId: string): Promise<void> {
    setError(null);
    try {
      const { error: apiError } = await client.PUT('/settings/agent/default-model', {
        params: { query: { remoteNode } },
        body: { modelId },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to set default model');
      }
      setDefaultModelId(modelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set default model');
    }
  }

  async function handleDeleteModel(): Promise<void> {
    if (!deletingModel) return;
    try {
      const { error: apiError } = await client.DELETE('/settings/agent/models/{modelId}', {
        params: { path: { modelId: deletingModel.id }, query: { remoteNode } },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to delete model');
      }
      setError(null);
      setDeletingModel(null);
      await fetchModels();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete model');
    }
  }

  const normalizedPolicy = useMemo(() => normalizeToolPolicy(toolPolicy, toolMetas), [toolPolicy, toolMetas]);

  useEffect(() => {
    const ruleCount = normalizedPolicy.bash?.rules?.length || 0;
    setBashRuleIds((prev) => {
      if (prev.length === ruleCount) {
        return prev;
      }
      if (prev.length > ruleCount) {
        return prev.slice(0, ruleCount);
      }
      return [...prev, ...buildBashRuleIDs(ruleCount - prev.length)];
    });
  }, [buildBashRuleIDs, normalizedPolicy.bash?.rules?.length]);

  function updateToolToggle(toolName: string, value: boolean): void {
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      return {
        ...normalized,
        tools: {
          ...normalized.tools,
          [toolName]: value,
        },
      };
    });
  }

  function updateBashPolicy<K extends keyof NonNullable<AgentToolPolicy['bash']>>(
    key: K,
    value: NonNullable<AgentToolPolicy['bash']>[K]
  ): void {
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      return {
        ...normalized,
        bash: {
          ...normalized.bash,
          [key]: value,
        },
      };
    });
  }

  function addBashRule(): void {
    const newRule: AgentBashRule = {
      name: '',
      pattern: '',
      action: AgentBashRuleAction.allow,
      enabled: true,
    };
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      return {
        ...normalized,
        bash: {
          ...normalized.bash,
          rules: [...(normalized.bash?.rules || []), newRule],
        },
      };
    });
    setBashRuleIds((prev) => [...prev, nextBashRuleId()]);
  }

  function updateBashRule(index: number, patch: Partial<AgentBashRule>): void {
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      const rules = [...(normalized.bash?.rules || [])];
      if (!rules[index]) return prev;
      rules[index] = { ...rules[index], ...patch };
      return {
        ...normalized,
        bash: {
          ...normalized.bash,
          rules,
        },
      };
    });
  }

  function removeBashRule(index: number): void {
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      const rules = [...(normalized.bash?.rules || [])];
      if (!rules[index]) return prev;
      rules.splice(index, 1);
      return {
        ...normalized,
        bash: {
          ...normalized.bash,
          rules,
        },
      };
    });
    setBashRuleIds((prev) => {
      if (index < 0 || index >= prev.length) return prev;
      return prev.filter((_, idx) => idx !== index);
    });
  }

  function moveBashRule(index: number, direction: -1 | 1): void {
    setToolPolicy((prev) => {
      const normalized = normalizeToolPolicy(prev, toolMetas);
      const rules = [...(normalized.bash?.rules || [])];
      const targetIndex = index + direction;
      if (targetIndex < 0 || targetIndex >= rules.length) return prev;
      const [moved] = rules.splice(index, 1);
      if (!moved) return prev;
      rules.splice(targetIndex, 0, moved);
      return {
        ...normalized,
        bash: {
          ...normalized.bash,
          rules,
        },
      };
    });
    setBashRuleIds((prev) => {
      const ids = [...prev];
      const targetIndex = index + direction;
      if (targetIndex < 0 || targetIndex >= ids.length) return prev;
      const [movedID] = ids.splice(index, 1);
      if (!movedID) return prev;
      ids.splice(targetIndex, 0, movedID);
      return ids;
    });
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
    <div className="space-y-4 max-w-7xl pb-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Agent Settings</h1>
          <p className="text-sm text-muted-foreground">
            Configure the AI assistant for workflow generation
          </p>
        </div>
        <Button
          onClick={handleSaveConfig}
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

      {/* General Settings */}
      <div className="card-obsidian p-4 space-y-4 max-w-xl">
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

        {souls.length > 0 && (
          <div className="space-y-1">
            <Label className="text-sm font-medium">Agent Personality</Label>
            <p className="text-xs text-muted-foreground">
              Select the soul that defines the agent&apos;s identity and communication style
            </p>
            <Select
              value={selectedSoulId || '__none__'}
              onValueChange={(value) => setSelectedSoulId(value === '__none__' ? undefined : value)}
            >
              <SelectTrigger className="h-8 text-xs max-w-[300px]">
                <SelectValue placeholder="Select soul" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__" className="text-xs">
                  Default (no soul)
                </SelectItem>
                {souls.map((s) => (
                  <SelectItem key={s.id} value={s.id} className="text-xs">
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      {/* Web Search */}
      <div className="card-obsidian p-4 space-y-4 max-w-xl">
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <Label htmlFor="web-search" className="text-sm font-medium">
              Web Search
            </Label>
            <p className="text-xs text-muted-foreground">
              Enable provider-native web search for agent sessions
            </p>
          </div>
          <Switch
            id="web-search"
            checked={webSearchEnabled}
            onCheckedChange={setWebSearchEnabled}
          />
        </div>

        {webSearchEnabled && (
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">Max Uses per Request</Label>
            <Input
              type="number"
              min={1}
              className="h-8 text-xs max-w-[200px]"
              placeholder="No limit"
              value={webSearchMaxUses ?? ''}
              onChange={(e) => {
                const val = e.target.value;
                setWebSearchMaxUses(val === '' ? undefined : parseInt(val, 10));
              }}
            />
          </div>
        )}
      </div>

      {/* Tool Permissions */}
      <div className="card-obsidian p-4 space-y-4">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Shield className="h-4 w-4 text-muted-foreground" />
          Tool Permissions
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          {toolMetas.map((tool) => (
            <div
              key={tool.name}
              className="rounded-md border border-border/60 p-3 flex items-start justify-between gap-3"
            >
              <div className="space-y-0.5 min-w-0">
                <p className="text-sm font-medium">{tool.label}</p>
                <p className="text-xs text-muted-foreground">{tool.description}</p>
              </div>
              <Switch
                checked={normalizedPolicy.tools?.[tool.name] ?? false}
                onCheckedChange={(checked) => updateToolToggle(tool.name, checked)}
              />
            </div>
          ))}
        </div>

        <div className="border border-border/60 rounded-md p-3 space-y-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-sm font-medium">Bash Command Policy</p>
              <p className="text-xs text-muted-foreground">
                Regex rules are checked top-down for each command segment.
              </p>
            </div>
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1">
              <Label className="text-xs text-muted-foreground">No Match Behavior</Label>
              <Select
                value={normalizedPolicy.bash?.defaultBehavior || AgentBashPolicyDefaultBehavior.deny}
                onValueChange={(value) =>
                  updateBashPolicy('defaultBehavior', value as AgentBashPolicyDefaultBehavior)
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={AgentBashPolicyDefaultBehavior.allow}>Allow</SelectItem>
                  <SelectItem value={AgentBashPolicyDefaultBehavior.deny}>Deny</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label className="text-xs text-muted-foreground">On Deny</Label>
              <Select
                value={normalizedPolicy.bash?.denyBehavior || AgentBashPolicyDenyBehavior.ask_user}
                onValueChange={(value) =>
                  updateBashPolicy('denyBehavior', value as AgentBashPolicyDenyBehavior)
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={AgentBashPolicyDenyBehavior.ask_user}>Ask User</SelectItem>
                  <SelectItem value={AgentBashPolicyDenyBehavior.block}>Block</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label className="text-xs text-muted-foreground">Rules (ordered)</Label>
              <Button size="sm" className="h-7 text-xs" variant="outline" onClick={addBashRule}>
                <Plus className="h-3.5 w-3.5 mr-1" />
                Add Rule
              </Button>
            </div>

            {(normalizedPolicy.bash?.rules || []).length === 0 ? (
              <div className="rounded-md border border-dashed border-border/80 p-3 text-xs text-muted-foreground">
                No rules defined. Behavior falls back to "No Match Behavior".
              </div>
            ) : (
              <div className="space-y-2">
                {(normalizedPolicy.bash?.rules || []).map((rule, index) => (
                  <div key={bashRuleIds[index] || `bash_rule_fallback_${index}`} className="rounded-md border border-border/60 p-2 space-y-2">
                    <div className="grid gap-2 md:grid-cols-[1fr,2fr,150px,auto] items-end">
                      <div className="space-y-1">
                        <Label className="text-xs text-muted-foreground">Name</Label>
                        <Input
                          value={rule.name || ''}
                          onChange={(e) => updateBashRule(index, { name: e.target.value })}
                          className="h-8 text-xs"
                          placeholder={`rule_${index + 1}`}
                        />
                      </div>
                      <div className="space-y-1">
                        <Label className="text-xs text-muted-foreground">Regex Pattern</Label>
                        <Input
                          value={rule.pattern}
                          onChange={(e) => updateBashRule(index, { pattern: e.target.value })}
                          className="h-8 text-xs font-mono"
                          placeholder="^git\\s+status$"
                        />
                      </div>
                      <div className="space-y-1">
                        <Label className="text-xs text-muted-foreground">Action</Label>
                        <Select
                          value={rule.action}
                          onValueChange={(value) =>
                            updateBashRule(index, { action: value as AgentBashRuleAction })
                          }
                        >
                          <SelectTrigger className="h-8 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value={AgentBashRuleAction.allow}>Allow</SelectItem>
                            <SelectItem value={AgentBashRuleAction.deny}>Deny</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="flex items-center justify-end gap-2">
                        <div className="flex items-center gap-2">
                          <Label className="text-xs text-muted-foreground">Enabled</Label>
                          <Switch
                            checked={rule.enabled ?? true}
                            onCheckedChange={(checked) => updateBashRule(index, { enabled: checked })}
                          />
                        </div>
                      </div>
                    </div>
                    <div className="flex justify-end gap-2">
                      <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => moveBashRule(index, -1)}>
                        Up
                      </Button>
                      <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => moveBashRule(index, 1)}>
                        Down
                      </Button>
                      <Button size="sm" variant="destructive" className="h-7 text-xs" onClick={() => removeBashRule(index)}>
                        Remove
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Models Table */}
      {enabled && (
        <div className="card-obsidian overflow-auto">
          <div className="flex items-center justify-between p-4 pb-2">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Bot className="h-4 w-4 text-muted-foreground" />
              Models
            </div>
            <Button
              onClick={() => setShowCreateModal(true)}
              size="sm"
              className="h-8"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              Add Model
            </Button>
          </div>

          <Table className="text-xs">
            <TableHeader>
              <TableRow>
                <TableHead className="w-[180px]">Name</TableHead>
                <TableHead className="w-[140px]">ID</TableHead>
                <TableHead className="w-[120px]">Provider</TableHead>
                <TableHead className="w-[180px]">Model</TableHead>
                <TableHead className="w-[100px]">API Key</TableHead>
                <TableHead className="w-[80px]">Default</TableHead>
                <TableHead className="w-[60px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className="text-center text-muted-foreground py-8"
                  >
                    No models configured. Add a model to get started.
                  </TableCell>
                </TableRow>
              ) : (
                models.map((m) => (
                  <TableRow key={m.id}>
                    <TableCell className="font-medium max-w-[200px]">
                      <div className="flex flex-col overflow-hidden">
                        <span className="truncate">{m.name}</span>
                        {m.description && (
                          <span className="text-xs text-muted-foreground truncate">
                            {m.description}
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="max-w-[160px]">
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded truncate block">
                        {m.id}
                      </code>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                        {m.provider}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-[180px]">
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded truncate block">
                        {m.model}
                      </code>
                    </TableCell>
                    <TableCell>
                      <span className={`text-xs ${m.apiKeyConfigured ? 'text-green-600' : 'text-muted-foreground'}`}>
                        {m.apiKeyConfigured ? 'Configured' : 'Not set'}
                      </span>
                    </TableCell>
                    <TableCell>
                      {m.id === defaultModelId && (
                        <span className="inline-flex items-center gap-1 text-xs text-amber-600">
                          <Star className="h-3 w-3 fill-current" />
                          Default
                        </span>
                      )}
                    </TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditingModel(m)}>
                            <Pencil className="h-4 w-4 mr-2" />
                            Edit
                          </DropdownMenuItem>
                          {m.id !== defaultModelId && (
                            <DropdownMenuItem onClick={() => handleSetDefault(m.id)}>
                              <Star className="h-4 w-4 mr-2" />
                              Set as Default
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem
                            onClick={() => setDeletingModel(m)}
                            className="text-destructive"
                          >
                            <Trash2 className="h-4 w-4 mr-2" />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create Model Modal */}
      <ModelFormModal
        open={showCreateModal}
        presets={presets}
        onClose={() => setShowCreateModal(false)}
        onSuccess={() => {
          setShowCreateModal(false);
          fetchModels();
        }}
      />

      {/* Edit Model Modal */}
      <ModelFormModal
        open={!!editingModel}
        model={editingModel || undefined}
        presets={presets}
        onClose={() => setEditingModel(null)}
        onSuccess={() => {
          setEditingModel(null);
          fetchModels();
        }}
      />

      {/* Delete Confirmation */}
      <ConfirmModal
        title="Delete Model"
        buttonText="Delete"
        visible={!!deletingModel}
        dismissModal={() => setDeletingModel(null)}
        onSubmit={handleDeleteModel}
      >
        <p>
          Are you sure you want to delete the model &quot;{deletingModel?.name}
          &quot;? This action cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}
