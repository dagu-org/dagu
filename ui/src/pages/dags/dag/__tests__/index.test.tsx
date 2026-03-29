// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import DAGDetails from '..';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/contexts/PageContext', () => ({
  usePageContext: () => ({
    setContext: vi.fn(),
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
  useLiveDAG: vi.fn(),
  useLiveDAGRuns: vi.fn(),
}));

vi.mock('../../../../features/dags/components/dag-details', () => ({
  DAGHeader: () => <div>dag header</div>,
  DAGDetailsContent: () => <div>dag content</div>,
}));

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (path: string, init?: unknown, config?: unknown) => unknown
  ) => void;
};

type QueryState = {
  dagTags: string[];
};

let queryState: QueryState;

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/dags/example.yaml']}>
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
        <Routes>
          <Route path="/dags/:fileName" element={<DAGDetails />} />
        </Routes>
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  queryState = {
    dagTags: ['workspace=ops'],
  };

  useQueryMock.mockImplementation((path) => {
    if (path === '/dags/{fileName}') {
      return {
        data: {
          dag: {
            name: 'example',
            tags: queryState.dagTags,
          },
          filePath: '/tmp/example.yaml',
          latestDAGRun: undefined,
        },
        mutate: vi.fn(),
      } as never;
    }

    return {
      data: undefined,
      mutate: vi.fn(),
    } as never;
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DAGDetails workspace boundary', () => {
  it('renders DAG content when the DAG matches the selected workspace', () => {
    renderPage();

    expect(screen.getByText('dag header')).toBeInTheDocument();
    expect(screen.getByText('dag content')).toBeInTheDocument();
  });

  it('renders a filtered-out state when the DAG does not match the selected workspace', () => {
    queryState.dagTags = ['workspace=other'];

    renderPage();

    expect(screen.getByText('DAG Not Available')).toBeInTheDocument();
    expect(
      screen.getByText('This DAG is outside the selected workspace.')
    ).toBeInTheDocument();
    expect(screen.queryByText('dag header')).not.toBeInTheDocument();
  });
});
