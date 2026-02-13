import React, { useCallback, useContext, useEffect, useState } from 'react';
import { Brain, Loader2, Save, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';
import ConfirmModal from '@/ui/ConfirmModal';

export default function AgentMemoryPage(): React.ReactNode {
  const client = useClient();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const { setTitle } = appBarContext;

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [globalMemory, setGlobalMemory] = useState('');
  const [dagNames, setDagNames] = useState<string[]>([]);
  const [memoryDir, setMemoryDir] = useState('');

  const [selectedDAG, setSelectedDAG] = useState<string | null>(null);
  const [dagContent, setDagContent] = useState('');
  const [isDagLoading, setIsDagLoading] = useState(false);
  const [isDagSaving, setIsDagSaving] = useState(false);

  const [deletingGlobal, setDeletingGlobal] = useState(false);
  const [deletingDAG, setDeletingDAG] = useState<string | null>(null);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  useEffect(() => {
    setTitle('Agent Memory');
  }, [setTitle]);

  const fetchMemory = useCallback(async () => {
    setIsLoading(true);
    try {
      const { data, error: apiError } = await client.GET('/settings/agent/memory', {
        params: { query: { remoteNode } },
      });
      if (apiError) throw new Error('Failed to fetch agent memory');
      setGlobalMemory(data.globalMemory ?? '');
      setDagNames(data.dagMemories ?? []);
      setMemoryDir(data.memoryDir ?? '');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load memory');
    } finally {
      setIsLoading(false);
    }
  }, [client, remoteNode]);

  useEffect(() => {
    fetchMemory();
  }, [fetchMemory]);

  async function handleSaveGlobal(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { error: apiError } = await client.PUT('/settings/agent/memory', {
        params: { query: { remoteNode } },
        body: { content: globalMemory },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to save memory');
      setSuccess('Global memory saved');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save memory');
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDeleteGlobal(): Promise<void> {
    try {
      const { error: apiError } = await client.DELETE('/settings/agent/memory', {
        params: { query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to clear memory');
      setGlobalMemory('');
      setDeletingGlobal(false);
      setSuccess('Global memory cleared');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to clear memory');
    }
  }

  async function handleSelectDAG(dagName: string): Promise<void> {
    setSelectedDAG(dagName);
    setIsDagLoading(true);
    setError(null);
    try {
      const { data, error: apiError } = await client.GET('/settings/agent/memory/dags/{dagName}', {
        params: { path: { dagName }, query: { remoteNode } },
      });
      if (apiError) throw new Error('Failed to load DAG memory');
      setDagContent(data.content);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load DAG memory');
      setSelectedDAG(null);
    } finally {
      setIsDagLoading(false);
    }
  }

  async function handleSaveDAG(): Promise<void> {
    if (!selectedDAG) return;
    setIsDagSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { error: apiError } = await client.PUT('/settings/agent/memory/dags/{dagName}', {
        params: { path: { dagName: selectedDAG }, query: { remoteNode } },
        body: { content: dagContent },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to save DAG memory');
      setSuccess(`Memory for "${selectedDAG}" saved`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save DAG memory');
    } finally {
      setIsDagSaving(false);
    }
  }

  async function handleDeleteDAG(): Promise<void> {
    if (!deletingDAG) return;
    try {
      const { error: apiError } = await client.DELETE('/settings/agent/memory/dags/{dagName}', {
        params: { path: { dagName: deletingDAG }, query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to delete DAG memory');
      setDagNames((prev) => prev.filter((n) => n !== deletingDAG));
      if (selectedDAG === deletingDAG) {
        setSelectedDAG(null);
        setDagContent('');
      }
      setDeletingDAG(null);
      setSuccess(`Memory for "${deletingDAG}" deleted`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete DAG memory');
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

  const globalLineCount = globalMemory.split('\n').length;

  return (
    <div className="space-y-4 max-w-7xl">
      <div>
        <h1 className="text-lg font-semibold">Agent Memory</h1>
        <p className="text-sm text-muted-foreground">
          View and manage the AI agent's persistent memory
        </p>
        {isLoading && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground mt-1">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Loading memory...
          </div>
        )}
        {memoryDir && (
          <p className="text-xs text-muted-foreground mt-1 font-mono">{memoryDir}</p>
        )}
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

      {/* Global Memory */}
      <div className="card-obsidian p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Brain className="h-4 w-4 text-muted-foreground" />
            Global Memory
          </div>
          <span className="text-xs text-muted-foreground">
            {globalLineCount}/200 lines
          </span>
        </div>

        <textarea
          value={globalMemory}
          onChange={(e) => setGlobalMemory(e.target.value)}
          disabled={isLoading}
          className="w-full h-64 p-3 text-sm font-mono bg-muted/50 border rounded-md resize-y focus:outline-none focus:ring-1 focus:ring-ring"
          placeholder="No global memory yet. The agent will write here when it learns something."
        />

        <div className="flex gap-2">
          <Button
            onClick={handleSaveGlobal}
            disabled={isSaving || isLoading}
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
                Save
              </>
            )}
          </Button>
          <Button
            onClick={() => setDeletingGlobal(true)}
            variant="outline"
            size="sm"
            className="h-8"
            disabled={!globalMemory || isLoading}
          >
            <Trash2 className="h-4 w-4 mr-1.5" />
            Clear
          </Button>
        </div>
      </div>

      {/* Per-DAG Memory */}
      <div className="card-obsidian p-4 space-y-3">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Brain className="h-4 w-4 text-muted-foreground" />
          Per-DAG Memory
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-4">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : dagNames.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4 text-center">
            No DAG-specific memories yet. The agent will create these when working with specific DAGs.
          </p>
        ) : (
          <div className="space-y-2">
            <div className="flex flex-wrap gap-2">
              {dagNames.map((name) => (
                <div
                  key={name}
                  role="button"
                  tabIndex={0}
                  onClick={() => handleSelectDAG(name)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleSelectDAG(name);
                    }
                  }}
                  className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md border transition-colors cursor-pointer ${
                    selectedDAG === name
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-muted/50 hover:bg-muted border-border'
                  }`}
                >
                  {name}
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeletingDAG(name);
                    }}
                    className="ml-1 hover:text-destructive"
                    title={`Delete memory for ${name}`}
                  >
                    <Trash2 className="h-3 w-3" />
                  </button>
                </div>
              ))}
            </div>

            {selectedDAG && (
              <div className="space-y-2 pt-2 border-t">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-muted-foreground">
                    Memory for: <code className="bg-muted px-1.5 py-0.5 rounded">{selectedDAG}</code>
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {dagContent.split('\n').length}/200 lines
                  </span>
                </div>

                {isDagLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                  </div>
                ) : (
                  <>
                    <textarea
                      value={dagContent}
                      onChange={(e) => setDagContent(e.target.value)}
                      className="w-full h-48 p-3 text-sm font-mono bg-muted/50 border rounded-md resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                    />
                    <Button
                      onClick={handleSaveDAG}
                      disabled={isDagSaving}
                      size="sm"
                      className="h-8"
                    >
                      {isDagSaving ? (
                        <>
                          <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                          Saving...
                        </>
                      ) : (
                        <>
                          <Save className="h-4 w-4 mr-1.5" />
                          Save
                        </>
                      )}
                    </Button>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Delete Global Confirmation */}
      <ConfirmModal
        title="Clear Global Memory"
        buttonText="Clear"
        visible={deletingGlobal}
        dismissModal={() => setDeletingGlobal(false)}
        onSubmit={handleDeleteGlobal}
      >
        <p>
          Are you sure you want to clear the global agent memory? This action cannot be undone.
        </p>
      </ConfirmModal>

      {/* Delete DAG Memory Confirmation */}
      <ConfirmModal
        title="Delete DAG Memory"
        buttonText="Delete"
        visible={!!deletingDAG}
        dismissModal={() => setDeletingDAG(null)}
        onSubmit={handleDeleteDAG}
      >
        <p>
          Are you sure you want to delete the memory for &quot;{deletingDAG}
          &quot;? This action cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}
