// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
