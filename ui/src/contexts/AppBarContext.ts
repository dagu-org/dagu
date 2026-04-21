// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import type { WorkspaceResponse } from './ConfigContext';
import {
  defaultWorkspaceSelection,
  type WorkspaceSelection,
} from '@/lib/workspace';

type AppBarContextType = {
  title: string;
  setTitle(val: string): void;
  remoteNodes: string[];
  setRemoteNodes(nodes: string[]): void;
  selectedRemoteNode: string;
  selectRemoteNode(val: string): void;
  workspaces?: WorkspaceResponse[];
  workspaceError?: Error | null;
  workspaceSelection?: WorkspaceSelection;
  selectWorkspace?(selection: WorkspaceSelection): void;
  createWorkspace?(name: string): Promise<void>;
  deleteWorkspace?(id: string): Promise<void>;
};

export const AppBarContext = React.createContext<AppBarContextType>({
  title: '',
  setTitle: () => {
    return;
  },
  selectedRemoteNode: '',
  remoteNodes: [],
  workspaces: [],
  workspaceError: null,
  workspaceSelection: defaultWorkspaceSelection(),
  selectWorkspace: () => {
    return;
  },
  setRemoteNodes: () => {
    return;
  },
  selectRemoteNode: () => {
    return;
  },
  createWorkspace: async () => {
    return;
  },
  deleteWorkspace: async () => {
    return;
  },
});
