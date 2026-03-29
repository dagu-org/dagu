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
    selectedWorkspace: workspace.selectedWorkspace,
    workspaceReady: workspace.workspaceReady,
    selectedTemplate,
    selectTemplate: setSelectedTemplate,
  };
}
