import React from 'react';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '../AppBarContext';
import { WorkspaceProvider, useWorkspace } from '../WorkspaceContext';
import { useClient, useQuery } from '@/hooks/api';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (path: string, init?: unknown) => unknown
  ) => void;
};
const useClientMock = useClient as unknown as {
  mockReturnValue: (value: unknown) => void;
};

type QueryState = {
  data?: { workspaces: Array<{ id: string; name: string }> };
  error?: Error;
  mutate?: () => Promise<unknown>;
};

const queryStates: Record<string, QueryState> = {};
const queryCalls: Array<{ path: string; init?: unknown }> = [];

function WorkspaceConsumer() {
  const { selectedWorkspace, workspaceReady, workspaces, selectWorkspace } =
    useWorkspace();

  return (
    <div>
      <div data-testid="selected">{selectedWorkspace || '(none)'}</div>
      <div data-testid="ready">{String(workspaceReady)}</div>
      <div data-testid="workspaces">{workspaces.map((ws) => ws.name).join(',')}</div>
      <button type="button" onClick={() => selectWorkspace('ops')}>
        select-ops
      </button>
      <button type="button" onClick={() => selectWorkspace('qa')}>
        select-qa
      </button>
    </div>
  );
}

function renderWorkspaceProvider(initialNode = 'local') {
  function Harness() {
    const [selectedRemoteNode, setSelectedRemoteNode] = React.useState(
      initialNode
    );

    return (
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: vi.fn(),
          remoteNodes: ['local', 'dev'],
          setRemoteNodes: vi.fn(),
          selectedRemoteNode,
          selectRemoteNode: setSelectedRemoteNode,
        }}
      >
        <WorkspaceProvider>
          <WorkspaceConsumer />
        </WorkspaceProvider>
        <button type="button" onClick={() => setSelectedRemoteNode('local')}>
          switch-local
        </button>
        <button type="button" onClick={() => setSelectedRemoteNode('dev')}>
          switch-dev
        </button>
      </AppBarContext.Provider>
    );
  }

  return render(<Harness />);
}

beforeEach(() => {
  localStorage.clear();
  queryCalls.length = 0;
  for (const key of Object.keys(queryStates)) {
    delete queryStates[key];
  }

  useClientMock.mockReturnValue({
    POST: vi.fn(),
    DELETE: vi.fn(),
  } as never);
  useQueryMock.mockImplementation((path, init) => {
    queryCalls.push({ path, init });
    if (path !== '/workspaces') {
      return { data: undefined, mutate: vi.fn() } as never;
    }
    const remoteNode =
      (
        init as {
          params?: { query?: { remoteNode?: string } };
        }
      )?.params?.query?.remoteNode ?? 'local';
    const state = queryStates[remoteNode] ?? {};
    return {
      data: state.data,
      error: state.error,
      mutate: state.mutate ?? vi.fn().mockResolvedValue(undefined),
    } as never;
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('WorkspaceProvider', () => {
  it('persists workspace selection separately for each remote node', () => {
    queryStates.local = {
      data: { workspaces: [{ id: 'ws-local', name: 'ops' }] },
    };
    queryStates.dev = {
      data: { workspaces: [{ id: 'ws-dev', name: 'qa' }] },
    };

    renderWorkspaceProvider();

    fireEvent.click(screen.getByRole('button', { name: 'select-ops' }));
    expect(screen.getByTestId('selected')).toHaveTextContent('ops');

    fireEvent.click(screen.getByRole('button', { name: 'switch-dev' }));
    expect(screen.getByTestId('selected')).toHaveTextContent('(none)');

    fireEvent.click(screen.getByRole('button', { name: 'select-qa' }));
    expect(screen.getByTestId('selected')).toHaveTextContent('qa');

    fireEvent.click(screen.getByRole('button', { name: 'switch-local' }));
    expect(screen.getByTestId('selected')).toHaveTextContent('ops');

    expect(
      JSON.parse(localStorage.getItem('dagu_selected_workspace_by_node') || '{}')
    ).toEqual({
      local: 'ops',
      dev: 'qa',
    });
  });

  it('derives the current selection synchronously from the active remote node', () => {
    localStorage.setItem(
      'dagu_selected_workspace_by_node',
      JSON.stringify({ local: 'ops', dev: 'qa' })
    );
    queryStates.local = {
      data: { workspaces: [{ id: 'ws-local', name: 'ops' }] },
    };
    queryStates.dev = {};

    renderWorkspaceProvider();

    expect(screen.getByTestId('selected')).toHaveTextContent('ops');
    expect(screen.getByTestId('ready')).toHaveTextContent('true');

    fireEvent.click(screen.getByRole('button', { name: 'switch-dev' }));

    expect(screen.getByTestId('selected')).toHaveTextContent('qa');
    expect(screen.getByTestId('ready')).toHaveTextContent('false');
    expect(queryCalls.at(-1)?.init).toEqual(
      expect.objectContaining({
        params: expect.objectContaining({
          query: { remoteNode: 'dev' },
        }),
      })
    );
  });

  it('clears an invalid stored selection after validating the workspace list', async () => {
    localStorage.setItem(
      'dagu_selected_workspace_by_node',
      JSON.stringify({ local: 'ops' })
    );
    queryStates.local = {
      data: { workspaces: [{ id: 'ws-local', name: 'dev' }] },
    };

    renderWorkspaceProvider();

    await waitFor(() =>
      expect(screen.getByTestId('selected')).toHaveTextContent('(none)')
    );
    expect(
      JSON.parse(localStorage.getItem('dagu_selected_workspace_by_node') || '{}')
    ).toEqual({});
  });

  it('does not carry local-node workspace state into another node while validation is pending', () => {
    localStorage.setItem(
      'dagu_selected_workspace_by_node',
      JSON.stringify({ local: 'ops' })
    );
    queryStates.local = {
      data: { workspaces: [{ id: 'ws-local', name: 'ops' }] },
    };
    queryStates.dev = {};

    renderWorkspaceProvider();

    fireEvent.click(screen.getByRole('button', { name: 'switch-dev' }));

    expect(screen.getByTestId('selected')).toHaveTextContent('(none)');
    expect(screen.getByTestId('ready')).toHaveTextContent('true');
  });
});
