import { useCallback, useContext, useState } from 'react';
import { useQuery, useClient } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';

const WORKSPACE_STORAGE_KEY = 'dagu_cockpit_workspace';

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();

  const { data, mutate } = useQuery('/workspaces', {
    params: { query: { remoteNode } },
  });

  const [selectedWorkspace, setSelectedWorkspace] = useState(
    () => localStorage.getItem(WORKSPACE_STORAGE_KEY) ?? ''
  );
  const [selectedTemplate, setSelectedTemplate] = useState('');

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
      const { error } = await client.POST('/workspaces', {
        params: { query: { remoteNode } },
        body: { name },
      });
      if (!error) {
        mutate();
        selectWorkspace(name);
      }
    },
    [client, remoteNode, mutate, selectWorkspace]
  );

  const deleteWorkspace = useCallback(
    async (id: string) => {
      const { error } = await client.DELETE('/workspaces/{workspaceId}', {
        params: { path: { workspaceId: id }, query: { remoteNode } },
      });
      if (!error) {
        mutate();
        selectWorkspace('');
      }
    },
    [client, remoteNode, mutate, selectWorkspace]
  );

  return {
    workspaces: data?.workspaces ?? [],
    selectedWorkspace,
    selectedTemplate,
    selectWorkspace,
    selectTemplate: setSelectedTemplate,
    createWorkspace,
    deleteWorkspace,
  };
}
