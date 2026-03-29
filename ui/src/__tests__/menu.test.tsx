import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AppBarContext } from '../contexts/AppBarContext';
import { mainListItems as MainListItems } from '../menu';

const selectWorkspace = vi.fn();
const selectRemoteNode = vi.fn();

const workspaceState = {
  workspaces: [
    { id: 'ws-ops', name: 'ops' },
    { id: 'ws-qa', name: 'qa' },
  ],
  workspaceError: null,
  selectedWorkspace: 'ops',
  workspaceReady: true,
  selectWorkspace,
  createWorkspace: vi.fn(),
  deleteWorkspace: vi.fn(),
  refreshWorkspaces: vi.fn(),
};

vi.mock('@/components/ui/select', async () => {
  const React = await import('react');

  type SelectContextValue = {
    onValueChange?: (value: string) => void;
  };

  const SelectContext = React.createContext<SelectContextValue>({});

  return {
    Select: ({
      children,
      onValueChange,
    }: {
      children: React.ReactNode;
      onValueChange?: (value: string) => void;
    }) => (
      <SelectContext.Provider value={{ onValueChange }}>
        {children}
      </SelectContext.Provider>
    ),
    SelectTrigger: ({
      children,
      ...props
    }: React.HTMLAttributes<HTMLDivElement>) => <div {...props}>{children}</div>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
    SelectItem: ({
      children,
      value,
    }: {
      children: React.ReactNode;
      value: string;
    }) => {
      const { onValueChange } = React.useContext(SelectContext);
      return (
        <button type="button" onClick={() => onValueChange?.(value)}>
          {children}
        </button>
      );
    },
  };
});

vi.mock('@/components/UserMenu', () => ({
  UserMenu: () => <div>user-menu</div>,
}));

vi.mock('@/contexts/AuthContext', () => ({
  useCanAccessSystemStatus: () => false,
  useCanManageWebhooks: () => false,
  useCanViewAuditLogs: () => false,
  useCanWrite: () => true,
  useIsAdmin: () => false,
}));

vi.mock('@/contexts/ConfigContext', () => ({
  useConfig: () => ({
    title: 'Dagu',
    agentEnabled: false,
    gitSyncEnabled: false,
    authMode: 'builtin',
    terminalEnabled: false,
    version: '1.0.0',
  }),
}));

vi.mock('@/hooks/useLicense', () => ({
  useHasFeature: () => false,
}));

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: { theme: 'dark' },
    updatePreference: vi.fn(),
  }),
}));

vi.mock('../features/agent', () => ({
  useAgentChatContext: () => ({
    toggleChat: vi.fn(),
    openChat: vi.fn(),
    setInitialInputValue: vi.fn(),
  }),
}));

vi.mock('../contexts/WorkspaceContext', () => ({
  useWorkspace: () => workspaceState,
}));

function renderMenu() {
  return render(
    <MemoryRouter>
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: vi.fn(),
          remoteNodes: ['local', 'dev'],
          setRemoteNodes: vi.fn(),
          selectedRemoteNode: 'local',
          selectRemoteNode,
        }}
      >
        <MainListItems isOpen={true} />
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  selectWorkspace.mockReset();
  selectRemoteNode.mockReset();
  workspaceState.selectedWorkspace = 'ops';
  workspaceState.workspaceReady = true;
  workspaceState.workspaces = [
    { id: 'ws-ops', name: 'ops' },
    { id: 'ws-qa', name: 'qa' },
  ];
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('MainListItems', () => {
  it('renders a workspace switcher in the sidebar and applies selections', () => {
    renderMenu();

    expect(screen.getAllByText('ops').length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole('button', { name: 'qa' }));
    expect(selectWorkspace).toHaveBeenCalledWith('qa');

    fireEvent.click(screen.getByRole('button', { name: 'All workspaces' }));
    expect(selectWorkspace).toHaveBeenCalledWith('');
  });
});
