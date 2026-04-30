// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { SearchStateProvider } from '@/contexts/SearchStateContext';
import { useQuery } from '@/hooks/api';
import { WorkspaceKind } from '@/lib/workspace';
import DagsPage from '../index';

vi.mock('@/components/SplitLayout', () => {
  return {
    PanelWidthContext: React.createContext<number | null>(null),
    default: ({
      leftPanel,
      rightPanel,
    }: {
      leftPanel: React.ReactNode;
      rightPanel: React.ReactNode;
    }) => (
      <div>
        <div>{leftPanel}</div>
        <div>{rightPanel}</div>
      </div>
    ),
  };
});

vi.mock('@/contexts/TabContext', () => ({
  TabProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useTabContext: () => ({
    tabs: [],
    activeTabId: null,
    selectDAG: vi.fn(),
    addTab: vi.fn(),
    closeTab: vi.fn(),
    getActiveFileName: () => '',
  }),
}));

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: {
      pageLimit: 200,
    },
    updatePreference: vi.fn(),
  }),
}));

vi.mock('@/features/dags/components/dag-details', () => ({
  DAGDetailsPanel: () => null,
}));

vi.mock('@/features/dags/components/dag-editor', () => ({
  DAGErrors: () => null,
}));

vi.mock('@/features/dags/components/dag-list', () => ({
  DAGTable: ({
    searchText,
    handleSearchTextChange,
  }: {
    searchText: string;
    handleSearchTextChange: (value: string) => void;
  }) => (
    <input
      aria-label="Search DAGs"
      value={searchText}
      onChange={(event) => handleSearchTextChange(event.target.value)}
    />
  ),
}));

vi.mock('@/features/dags/components/dag-list/DAGListHeader', () => ({
  default: () => null,
}));

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/hooks/useDAGsListSSE', () => ({
  useDAGsListSSE: () => ({
    isConnected: true,
    shouldUseFallback: false,
  }),
}));

vi.mock('@/hooks/useSSECacheSync', () => ({
  sseFallbackOptions: () => ({}),
  useSSECacheSync: () => undefined,
}));

type QueryCall = {
  path: string;
  init: unknown;
  config: unknown;
};

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (path: string, init?: unknown, config?: unknown) => unknown
  ) => void;
};

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    apiURL: '/api/v1',
    basePath: '/',
    title: 'Dagu',
    navbarColor: '',
    tz: 'UTC',
    tzOffsetInSec: 0,
    version: 'test',
    maxDashboardPageLimit: 100,
    remoteNodes: 'local,remote-a',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    agentEnabled: false,
    updateAvailable: false,
    latestVersion: '',
    permissions: {
      writeDags: true,
      runDags: true,
    },
    license: {
      valid: true,
      plan: 'community',
      expiry: '',
      features: [],
      gracePeriod: false,
      community: true,
      source: 'test',
      warningCode: '',
    },
    paths: {
      dagsDir: '',
      logDir: '',
      suspendFlagsDir: '',
      adminLogsDir: '',
      baseConfig: '',
      dagRunsDir: '',
      queueDir: '',
      procDir: '',
      serviceRegistryDir: '',
      configFileUsed: '',
      gitSyncDir: '',
      auditLogsDir: '',
    },
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/dags']}>
      <ConfigContext.Provider value={makeConfig()}>
        <SearchStateProvider>
          <AppBarContext.Provider
            value={{
              title: '',
              setTitle: () => undefined,
              remoteNodes: ['local', 'remote-a'],
              setRemoteNodes: () => undefined,
              selectedRemoteNode: 'remote-a',
              selectRemoteNode: () => undefined,
              workspaces: [],
              workspaceSelection: { kind: WorkspaceKind.all },
              selectWorkspace: () => undefined,
            }}
          >
            <DagsPage />
          </AppBarContext.Provider>
        </SearchStateProvider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('DagsPage', () => {
  const calls: QueryCall[] = [];

  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    sessionStorage.clear();
    calls.length = 0;

    useQueryMock.mockImplementation((path, init, config) => {
      calls.push({ path, init, config });

      if (path === '/dags/labels') {
        return {
          data: { labels: [] },
          isLoading: false,
          mutate: vi.fn(),
        };
      }

      if (path === '/dags') {
        const query = (
          init as {
            params?: { query?: { name?: string } };
          }
        )?.params?.query;
        const name = query?.name ?? '';
        const keepPreviousData = Boolean(
          (config as { keepPreviousData?: boolean } | undefined)
            ?.keepPreviousData
        );

        return {
          data: {
            dags: [
              {
                fileName: 'demo.yaml',
                dag: {
                  name: 'demo',
                },
                latestDAGRun: {},
              },
            ],
            errors: [],
            pagination: {
              totalPages: 1,
            },
          },
          isLoading: name.length > 0,
          mutate: vi.fn(),
          ...(name.length > 0 && !keepPreviousData ? { data: undefined } : {}),
        };
      }

      return {
        data: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('keeps the search input focused while incremental search refreshes results', async () => {
    renderPage();

    const input = screen.getByRole('textbox', { name: 'Search DAGs' });
    input.focus();
    expect(input).toHaveFocus();

    await act(async () => {
      fireEvent.change(input, { target: { value: 'demo' } });
      vi.advanceTimersByTime(500);
    });

    const latestDagsCall = [...calls]
      .reverse()
      .find((call) => call.path === '/dags');
    expect(latestDagsCall).toBeDefined();
    expect(latestDagsCall?.init).toEqual(
      expect.objectContaining({
        params: {
          query: expect.objectContaining({
            name: 'demo',
          }),
        },
      })
    );

    expect(screen.getByRole('textbox', { name: 'Search DAGs' })).toHaveFocus();
  });
});
