// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { SearchStateProvider } from '@/contexts/SearchStateContext';
import { useQuery } from '@/hooks/api';
import DAGs from '..';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: {
      pageLimit: 50,
    },
    updatePreference: vi.fn(),
  }),
}));

vi.mock('@/contexts/WorkspaceContext', () => ({
  useWorkspace: () => ({
    selectedWorkspace: 'ops',
    workspaceReady: true,
  }),
}));

vi.mock('@/hooks/useAppLive', () => ({
  liveFallbackOptions: () => ({}),
  useLiveConnection: () => ({}),
  useLiveDAGsList: vi.fn(),
}));

vi.mock('../../../components/SplitLayout', () => ({
  default: ({
    leftPanel,
    rightPanel,
  }: {
    leftPanel: React.ReactNode;
    rightPanel?: React.ReactNode;
  }) => (
    <div>
      <div>{leftPanel}</div>
      <div>{rightPanel}</div>
    </div>
  ),
}));

vi.mock('../../../features/dags/components/dag-list/DAGListHeader', () => ({
  default: () => <div>dag list header</div>,
}));

vi.mock('../../../features/dags/components/dag-editor', () => ({
  DAGErrors: () => null,
}));

vi.mock('../../../features/dags/components/dag-details', () => ({
  DAGDetailsPanel: () => <div>dag details panel</div>,
}));

vi.mock('../../../features/dags/components/dag-list', () => ({
  DAGTable: ({
    searchTags,
    handleSearchTagsChange,
  }: {
    searchTags: string[];
    handleSearchTagsChange: (tags: string[]) => void;
  }) => (
    <div>
      <div data-testid="search-tags">{searchTags.join(',')}</div>
      <button
        type="button"
        onClick={() => handleSearchTagsChange(['workspace=other'])}
      >
        add-workspace-tag
      </button>
    </div>
  ),
}));

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (path: string, init?: unknown, config?: unknown) => unknown
  ) => void;
};

const queryCalls: Array<{ path: string; init?: unknown }> = [];

function latestQueryCall(path: string) {
  const calls = queryCalls.filter((call) => call.path === path);
  return calls[calls.length - 1];
}

function renderPage(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: vi.fn(),
          remoteNodes: ['local'],
          setRemoteNodes: vi.fn(),
          selectedRemoteNode: 'local',
          selectRemoteNode: vi.fn(),
        }}
      >
        <SearchStateProvider>
          <DAGs />
        </SearchStateProvider>
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  sessionStorage.clear();
  queryCalls.length = 0;

  useQueryMock.mockImplementation((path, init) => {
    queryCalls.push({ path, init });
    if (path === '/dags') {
      return {
        data: {
          dags: [],
          errors: [],
          pagination: { totalPages: 1 },
        },
        mutate: vi.fn(),
        isLoading: false,
      } as never;
    }
    return { data: undefined, mutate: vi.fn(), isLoading: false } as never;
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DAG definitions workspace tag sanitization', () => {
  it('strips workspace tags restored from the URL before applying the global workspace filter', async () => {
    renderPage('/dags?tags=workspace=other,env=prod');

    await waitFor(() =>
      expect(latestQueryCall('/dags')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'env=prod,workspace=ops',
            }),
          }),
        })
      )
    );
    expect(screen.getByTestId('search-tags')).toHaveTextContent('env=prod');
  });

  it('strips workspace tags restored from session state before applying the global workspace filter', async () => {
    sessionStorage.setItem(
      'dagu.searchState',
      JSON.stringify({
        'dagDefinitions:local': {
          searchText: '',
          searchTags: ['workspace=other', 'env=prod'],
          page: 1,
          sortField: 'name',
          sortOrder: 'asc',
        },
      })
    );

    renderPage('/dags');

    await waitFor(() =>
      expect(latestQueryCall('/dags')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'env=prod,workspace=ops',
            }),
          }),
        })
      )
    );
    expect(screen.getByTestId('search-tags')).toHaveTextContent('env=prod');
  });

  it('rejects workspace tags from the tag change handler', async () => {
    renderPage('/dags');

    fireEvent.click(screen.getByRole('button', { name: 'add-workspace-tag' }));

    await waitFor(() =>
      expect(latestQueryCall('/dags')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'workspace=ops',
            }),
          }),
        })
      )
    );
    expect(screen.getByTestId('search-tags')).toHaveTextContent('');
  });
});
