import { useContext, useEffect, useState } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const selectedWorkspace = appBarContext.selectedWorkspace || '';
  const [selectedTemplate, setSelectedTemplate] = useState('');

  useEffect(() => {
    setSelectedTemplate('');
  }, [remoteNode, selectedWorkspace]);

  return {
    selectedWorkspace,
    selectedTemplate,
    selectTemplate: setSelectedTemplate,
  };
}
