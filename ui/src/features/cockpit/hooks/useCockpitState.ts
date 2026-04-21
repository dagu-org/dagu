// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { useContext, useEffect, useState } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { workspaceSelectionKey } from '@/lib/workspace';

export function useCockpitState() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const selectedWorkspace = appBarContext.selectedWorkspace || '';
  const workspaceKey = workspaceSelectionKey(appBarContext.workspaceSelection);
  const [selectedTemplate, setSelectedTemplate] = useState('');

  useEffect(() => {
    setSelectedTemplate('');
  }, [remoteNode, workspaceKey]);

  return {
    selectedWorkspace,
    workspaceKey,
    selectedTemplate,
    selectTemplate: setSelectedTemplate,
  };
}
