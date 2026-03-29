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
import DAGRuns from '..';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/contexts/ConfigContext', () => ({
  useConfig: () => ({
    tzOffsetInSec: 0,
  }),
}));

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: {
      dagRunsViewMode: 'list',
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
  useLiveDAGRuns: vi.fn(),
}));

vi.mock('../../../features/dag-runs/components/dag-run-list/DAGRunGroupedView', () => ({
  default: () => <div>grouped view</div>,
}));

vi.mock('../../../features/dag-runs/components/dag-run-list/DAGRunTable', () => ({
  default: () => <div>dag run table</div>,
}));

vi.mock('../../../features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: () => null,
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
          <DAGRuns />
        </SearchStateProvider>
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  sessionStorage.clear();
  queryCalls.length = 0;
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });

  useQueryMock.mockImplementation((path, init) => {
    queryCalls.push({ path, init });
    if (path === '/dags/tags') {
      return { data: { tags: ['env=prod', 'workspace=ops'] } } as never;
    }
    if (path === '/dag-runs') {
      return {
        data: { dagRuns: [] },
        mutate: vi.fn(),
      } as never;
    }
    return { data: undefined, mutate: vi.fn() } as never;
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DAGRuns workspace tag sanitization', () => {
  it('strips workspace tags restored from the URL before applying the global workspace filter', async () => {
    renderPage('/dag-runs?tags=workspace=other,env=prod');

    await waitFor(() =>
      expect(latestQueryCall('/dag-runs')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'env=prod,workspace=ops',
            }),
          }),
        })
      )
    );
  });

  it('strips workspace tags restored from session state before applying the global workspace filter', async () => {
    sessionStorage.setItem(
      'dagu.searchState',
      JSON.stringify({
        'dagRuns:local': {
          searchText: '',
          dagRunId: '',
          status: 'all',
          tags: ['workspace=other', 'env=prod'],
          fromDate: '2026-03-29T00:00',
          toDate: undefined,
          dateRangeMode: 'preset',
          datePreset: 'today',
          specificPeriod: 'date',
          specificValue: '2026-03-29',
        },
      })
    );

    renderPage('/dag-runs');

    await waitFor(() =>
      expect(latestQueryCall('/dag-runs')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'env=prod,workspace=ops',
            }),
          }),
        })
      )
    );
  });

  it('rejects workspace tags entered through the tag combobox', async () => {
    renderPage('/dag-runs');

    const input = screen.getByPlaceholderText('Filter by tags...');
    fireEvent.change(input, { target: { value: 'workspace=other' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    await waitFor(() =>
      expect(latestQueryCall('/dag-runs')?.init).toEqual(
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({
              tags: 'workspace=ops',
            }),
          }),
        })
      )
    );
  });
});
