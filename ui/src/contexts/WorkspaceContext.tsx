import React from 'react';
import { useQuery, useClient } from '@/hooks/api';
import { AppBarContext } from './AppBarContext';
import type { components } from '@/api/v1/schema';

type WorkspaceResponse = components['schemas']['WorkspaceResponse'];

type WorkspaceContextValue = {
  workspaces: WorkspaceResponse[];
  workspaceError: Error | null;
  selectedWorkspace: string;
  workspaceReady: boolean;
  selectWorkspace: (name: string) => void;
  createWorkspace: (name: string) => Promise<void>;
  deleteWorkspace: (id: string) => Promise<void>;
  refreshWorkspaces: () => Promise<unknown>;
};

const WORKSPACE_STORAGE_KEY = 'dagu_selected_workspace_by_node';
const LEGACY_WORKSPACE_STORAGE_KEY = 'dagu_cockpit_workspace';

const WorkspaceContext = React.createContext<WorkspaceContextValue | null>(
  null
);

function sortWorkspaces(workspaces: WorkspaceResponse[]): WorkspaceResponse[] {
  return [...workspaces].sort((a, b) => a.name.localeCompare(b.name));
}

function getInitialSelectedWorkspaceByNode(): Record<string, string> {
  if (typeof window === 'undefined') {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(WORKSPACE_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        return Object.fromEntries(
          Object.entries(parsed).filter(
            ([, value]) => typeof value === 'string' && value
          )
        );
      }
    }
  } catch (error) {
    console.warn('Failed to read workspace selection from localStorage', error);
  }

  const legacyWorkspace = window.localStorage.getItem(
    LEGACY_WORKSPACE_STORAGE_KEY
  );
  return legacyWorkspace ? { local: legacyWorkspace } : {};
}

function persistSelectedWorkspaceByNode(
  selectedWorkspaceByNode: Record<string, string>
) {
  if (typeof window === 'undefined') {
    return;
  }

  if (Object.keys(selectedWorkspaceByNode).length === 0) {
    window.localStorage.removeItem(WORKSPACE_STORAGE_KEY);
    window.localStorage.removeItem(LEGACY_WORKSPACE_STORAGE_KEY);
    return;
  }

  window.localStorage.setItem(
    WORKSPACE_STORAGE_KEY,
    JSON.stringify(selectedWorkspaceByNode)
  );
  window.localStorage.removeItem(LEGACY_WORKSPACE_STORAGE_KEY);
}

export function WorkspaceProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const [selectedWorkspaceByNode, setSelectedWorkspaceByNode] = React.useState<
    Record<string, string>
  >(() => getInitialSelectedWorkspaceByNode());
  const [workspaceError, setWorkspaceError] = React.useState<Error | null>(
    null
  );

  const {
    data,
    error,
    mutate,
  } = useQuery('/workspaces', {
    params: {
      query: {
        remoteNode,
      },
    },
  });

  const selectedWorkspace = selectedWorkspaceByNode[remoteNode] ?? '';

  const workspaces = React.useMemo(
    () => sortWorkspaces(data?.workspaces ?? []),
    [data?.workspaces]
  );

  const workspaceReady = !selectedWorkspace || !!data?.workspaces || !!error;

  const updateSelectedWorkspaceByNode = React.useCallback(
    (
      next:
        | Record<string, string>
        | ((prev: Record<string, string>) => Record<string, string>)
    ) => {
      setSelectedWorkspaceByNode((prev) => {
        const resolved = typeof next === 'function' ? next(prev) : next;
        persistSelectedWorkspaceByNode(resolved);
        return resolved;
      });
    },
    []
  );

  React.useEffect(() => {
    if (!error) {
      return;
    }
    setWorkspaceError(
      error instanceof Error
        ? error
        : new Error('Failed to load workspaces')
    );
  }, [error]);

  React.useEffect(() => {
    if (!data?.workspaces) {
      return;
    }

    setWorkspaceError(null);

    if (!selectedWorkspace) {
      return;
    }

    const stillExists = data.workspaces.some(
      (workspace) => workspace.name === selectedWorkspace
    );
    if (stillExists) {
      return;
    }

    updateSelectedWorkspaceByNode((prev) => {
      if (!prev[remoteNode]) {
        return prev;
      }
      const next = { ...prev };
      delete next[remoteNode];
      return next;
    });
  }, [data?.workspaces, remoteNode, selectedWorkspace, updateSelectedWorkspaceByNode]);

  const selectWorkspace = React.useCallback(
    (name: string) => {
      updateSelectedWorkspaceByNode((prev) => {
        if (!name) {
          if (!(remoteNode in prev)) {
            return prev;
          }
          const next = { ...prev };
          delete next[remoteNode];
          return next;
        }

        if (prev[remoteNode] === name) {
          return prev;
        }

        return {
          ...prev,
          [remoteNode]: name,
        };
      });
    },
    [remoteNode, updateSelectedWorkspaceByNode]
  );

  const createWorkspace = React.useCallback(
    async (name: string) => {
      if (!name) {
        return;
      }

      setWorkspaceError(null);
      const { data: createdWorkspace, error: createError } = await client.POST(
        '/workspaces',
        {
          params: { query: { remoteNode } },
          body: { name },
        }
      );
      if (createError) {
        const nextError = new Error(
          createError.message || 'Failed to create workspace'
        );
        console.error(nextError);
        setWorkspaceError(nextError);
        return;
      }

      if (createdWorkspace) {
        selectWorkspace(createdWorkspace.name);
      }
      await mutate();
    },
    [client, mutate, remoteNode, selectWorkspace]
  );

  const deleteWorkspace = React.useCallback(
    async (id: string) => {
      setWorkspaceError(null);
      const { error: deleteError } = await client.DELETE(
        '/workspaces/{workspaceId}',
        {
          params: { path: { workspaceId: id }, query: { remoteNode } },
        }
      );
      if (deleteError) {
        const nextError = new Error(
          deleteError.message || 'Failed to delete workspace'
        );
        console.error(nextError);
        setWorkspaceError(nextError);
        return;
      }

      selectWorkspace('');
      await mutate();
    },
    [client, mutate, remoteNode, selectWorkspace]
  );

  const refreshWorkspaces = React.useCallback(async () => mutate(), [mutate]);

  const value = React.useMemo<WorkspaceContextValue>(
    () => ({
      workspaces,
      workspaceError,
      selectedWorkspace,
      workspaceReady,
      selectWorkspace,
      createWorkspace,
      deleteWorkspace,
      refreshWorkspaces,
    }),
    [
      createWorkspace,
      deleteWorkspace,
      refreshWorkspaces,
      selectWorkspace,
      selectedWorkspace,
      workspaceError,
      workspaceReady,
      workspaces,
    ]
  );

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace() {
  const context = React.useContext(WorkspaceContext);
  if (!context) {
    throw new Error('useWorkspace must be used within a WorkspaceProvider');
  }
  return context;
}

export function useOptionalWorkspace() {
  return React.useContext(WorkspaceContext);
}
