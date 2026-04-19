import React from 'react';
import type { WorkspaceResponse } from './ConfigContext';

type AppBarContextType = {
  title: string;
  setTitle(val: string): void;
  remoteNodes: string[];
  setRemoteNodes(nodes: string[]): void;
  selectedRemoteNode: string;
  selectRemoteNode(val: string): void;
  workspaces?: WorkspaceResponse[];
  workspaceError?: Error | null;
  selectedWorkspace?: string;
  selectWorkspace?(val: string): void;
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
  setRemoteNodes: () => {
    return;
  },
  selectRemoteNode: () => {
    return;
  },
  selectedWorkspace: '',
  selectWorkspace: () => {
    return;
  },
  createWorkspace: async () => {
    return;
  },
  deleteWorkspace: async () => {
    return;
  },
});
