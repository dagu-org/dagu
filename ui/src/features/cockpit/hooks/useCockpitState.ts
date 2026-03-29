import { useContext, useEffect, useState } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useWorkspace } from '@/contexts/WorkspaceContext';

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const workspace = useWorkspace();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const [selectedTemplate, setSelectedTemplate] = useState('');

  useEffect(() => {
    setSelectedTemplate('');
  }, [remoteNode, workspace.selectedWorkspace]);

  return {
    workspaces: workspace.workspaces,
    workspaceError: workspace.workspaceError,
    selectedWorkspace: workspace.selectedWorkspace,
    workspaceReady: workspace.workspaceReady,
    selectedTemplate,
    selectWorkspace: workspace.selectWorkspace,
    selectTemplate: setSelectedTemplate,
    createWorkspace: workspace.createWorkspace,
    deleteWorkspace: workspace.deleteWorkspace,
  };
}
