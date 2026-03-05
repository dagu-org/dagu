import { useCallback, useContext, useEffect, useState } from 'react';
import { useQuery, useClient } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();

  const { data, mutate } = useQuery('/workspaces', {
    params: { query: { remoteNode } },
  });

  const [selectedWorkspace, setSelectedWorkspace] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');

  // Auto-select first workspace on initial load
  useEffect(() => {
    if (!selectedWorkspace && data?.workspaces?.length) {
      setSelectedWorkspace(data.workspaces[0]!.name);
    }
  }, [data?.workspaces]);

  const createWorkspace = useCallback(
    async (name: string) => {
      if (!name) return;
      const { error } = await client.POST('/workspaces', {
        params: { query: { remoteNode } },
        body: { name },
      });
      if (!error) {
        mutate();
        setSelectedWorkspace(name);
      }
    },
    [client, remoteNode, mutate]
  );

  const deleteWorkspace = useCallback(
    async (id: string) => {
      await client.DELETE('/workspaces/{workspaceId}', {
        params: { path: { workspaceId: id }, query: { remoteNode } },
      });
      mutate();
      setSelectedWorkspace('');
    },
    [client, remoteNode, mutate]
  );

  return {
    workspaces: data?.workspaces ?? [],
    selectedWorkspace,
    selectedTemplate,
    selectWorkspace: setSelectedWorkspace,
    selectTemplate: setSelectedTemplate,
    createWorkspace,
    deleteWorkspace,
  };
}
