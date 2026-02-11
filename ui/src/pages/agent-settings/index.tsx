import React, { useCallback, useContext, useEffect, useState } from 'react';
import { Bot, Loader2, MoreHorizontal, Pencil, Plus, Save, Star, Trash2 } from 'lucide-react';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Label } from '@/components/ui/label';
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
import { useConfig } from '@/contexts/ConfigContext';
import { getAuthHeaders } from '@/lib/authHeaders';
import ConfirmModal from '@/ui/ConfirmModal';
import { ModelFormModal } from './ModelFormModal';

type ModelConfig = components['schemas']['ModelConfigResponse'];
type ModelPreset = components['schemas']['ModelPreset'];

export default function AgentSettingsPage(): React.ReactNode {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [enabled, setEnabled] = useState(false);
  const [defaultModelId, setDefaultModelId] = useState<string | undefined>();
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [presets, setPresets] = useState<ModelPreset[]>([]);

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingModel, setEditingModel] = useState<ModelConfig | null>(null);
  const [deletingModel, setDeletingModel] = useState<ModelConfig | null>(null);

  const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');

  useEffect(() => {
    appBarContext.setTitle('Agent Settings');
  }, [appBarContext]);

  const fetchConfig = useCallback(async () => {
    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent?remoteNode=${remoteNode}`,
        { headers: getAuthHeaders() }
      );
      if (!response.ok) throw new Error('Failed to fetch agent configuration');
      const data = await response.json();
      setEnabled(data.enabled ?? false);
      setDefaultModelId(data.defaultModelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load configuration');
    }
  }, [config.apiURL, remoteNode]);

  const fetchModels = useCallback(async () => {
    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent/models?remoteNode=${remoteNode}`,
        { headers: getAuthHeaders() }
      );
      if (!response.ok) throw new Error('Failed to fetch models');
      const data = await response.json();
      setModels(data.models || []);
      setDefaultModelId(data.defaultModelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load models');
    }
  }, [config.apiURL, remoteNode]);

  const fetchPresets = useCallback(async () => {
    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent/model-presets?remoteNode=${remoteNode}`,
        { headers: getAuthHeaders() }
      );
      if (!response.ok) return;
      const data = await response.json();
      setPresets(data.presets || []);
    } catch {
      // Presets are optional, don't show error
    }
  }, [config.apiURL, remoteNode]);

  useEffect(() => {
    async function load() {
      await Promise.all([fetchConfig(), fetchModels(), fetchPresets()]);
      setIsLoading(false);
    }
    load();
  }, [fetchConfig, fetchModels, fetchPresets]);

  async function handleSaveConfig(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent?remoteNode=${remoteNode}`,
        {
          method: 'PATCH',
          headers: getAuthHeaders(),
          body: JSON.stringify({ enabled, defaultModelId }),
        }
      );

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to save configuration');
      }

      const data = await response.json();
      setEnabled(data.enabled ?? false);
      setDefaultModelId(data.defaultModelId);
      setSuccess('Configuration saved successfully');

      setTimeout(() => window.location.reload(), 500);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save configuration');
    } finally {
      setIsSaving(false);
    }
  }

  async function handleSetDefault(modelId: string): Promise<void> {
    setError(null);
    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent/default-model?remoteNode=${remoteNode}`,
        {
          method: 'PUT',
          headers: getAuthHeaders(),
          body: JSON.stringify({ modelId }),
        }
      );
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to set default model');
      }
      setDefaultModelId(modelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set default model');
    }
  }

  async function handleDeleteModel(): Promise<void> {
    if (!deletingModel) return;
    try {
      const response = await fetch(
        `${config.apiURL}/settings/agent/models/${deletingModel.id}?remoteNode=${remoteNode}`,
        {
          method: 'DELETE',
          headers: getAuthHeaders(),
        }
      );
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to delete model');
      }
      setError(null);
      setDeletingModel(null);
      fetchModels();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete model');
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

        <div className="pt-2">
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
                    <TableCell className="font-medium">
                      <div className="flex flex-col">
                        <span>{m.name}</span>
                        {m.description && (
                          <span className="text-xs text-muted-foreground">
                            {m.description}
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                        {m.id}
                      </code>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                        {m.provider}
                      </span>
                    </TableCell>
                    <TableCell>
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
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
