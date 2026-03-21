import { useCallback, useContext, useEffect, useState } from 'react';
import type { components } from '@/api/v1/schema';
import { useClient } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig, useUpdateConfig } from '@/contexts/ConfigContext';

const WORKSPACE_STORAGE_KEY = 'dagu_cockpit_workspace';
type WorkspaceResponse = components['schemas']['WorkspaceResponse'];

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const config = useConfig();
  const updateConfig = useUpdateConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();

  const [selectedWorkspace, setSelectedWorkspace] = useState(
    () => localStorage.getItem(WORKSPACE_STORAGE_KEY) ?? ''
  );
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [workspaces, setWorkspaces] = useState<WorkspaceResponse[]>(
    () => config.initialWorkspaces ?? []
  );
  const [workspaceError, setWorkspaceError] = useState<Error | null>(null);

  const applyWorkspaces = useCallback(
    (
      next:
        | WorkspaceResponse[]
        | ((prev: WorkspaceResponse[]) => WorkspaceResponse[])
    ) => {
      setWorkspaces((prev) => {
        const resolved = typeof next === 'function' ? next(prev) : next;
        updateConfig({ initialWorkspaces: resolved });
        return resolved;
      });
    },
    [updateConfig]
  );

  useEffect(() => {
    setWorkspaces(config.initialWorkspaces ?? []);
  }, [config.initialWorkspaces]);

  useEffect(() => {
    setSelectedTemplate('');
  }, [remoteNode]);

  const selectWorkspace = useCallback((name: string) => {
    setSelectedWorkspace(name);
    if (name) {
      localStorage.setItem(WORKSPACE_STORAGE_KEY, name);
    } else {
      localStorage.removeItem(WORKSPACE_STORAGE_KEY);
    }
  }, []);

  const createWorkspace = useCallback(
    async (name: string) => {
      if (!name) return;
      setWorkspaceError(null);
      const { data, error } = await client.POST('/workspaces', {
        params: { query: { remoteNode } },
        body: { name },
      });
      if (error) {
        const nextError = new Error(error.message || 'Failed to create workspace');
        console.error(nextError);
        setWorkspaceError(nextError);
        return;
      }
      if (data) {
        applyWorkspaces((prev) => {
          const next = [...prev.filter((ws) => ws.id !== data.id), data];
          return next.sort((a, b) => a.name.localeCompare(b.name));
        });
        selectWorkspace(data.name);
      }
    },
    [applyWorkspaces, client, remoteNode, selectWorkspace]
  );

  const deleteWorkspace = useCallback(
    async (id: string) => {
      setWorkspaceError(null);
      const { error } = await client.DELETE('/workspaces/{workspaceId}', {
        params: { path: { workspaceId: id }, query: { remoteNode } },
      });
      if (error) {
        const nextError = new Error(error.message || 'Failed to delete workspace');
        console.error(nextError);
        setWorkspaceError(nextError);
        return;
      }
      applyWorkspaces((prev) => prev.filter((ws) => ws.id !== id));
      selectWorkspace('');
    },
    [applyWorkspaces, client, remoteNode, selectWorkspace]
  );

  return {
    workspaces,
    workspaceError,
    selectedWorkspace,
    selectedTemplate,
    selectWorkspace,
    selectTemplate: setSelectedTemplate,
    createWorkspace,
    deleteWorkspace,
  };
}
