// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { PageContextProvider } from '@/contexts/PageContext';
import { useQuery } from '@/hooks/api';
import { useDAGSSE } from '@/hooks/useDAGSSE';
import DAGDetailsPanel from '../DAGDetailsPanel';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/hooks/useDAGSSE', () => ({
  useDAGSSE: vi.fn(),
}));

vi.mock('@/hooks/useSSECacheSync', () => ({
  sseFallbackOptions: vi.fn(() => ({})),
  useSSECacheSync: vi.fn(),
}));

vi.mock('../DAGDetailsContent', () => ({
  default: ({
    dag,
    activeTab,
    editorHints,
  }: {
    dag: { name: string };
    activeTab: string;
    editorHints?: { inheritedCustomStepTypes?: unknown[] };
  }) => (
    <div>
      <div>
        Previewing {dag.name} [{activeTab}]
      </div>
      <div>Inherited hints: {editorHints?.inheritedCustomStepTypes?.length ?? 0}</div>
    </div>
  ),
}));

const appBarValue = {
  title: 'DAGs',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const liveState = {
  data: null,
  error: null,
  isConnected: false,
  isConnecting: false,
  shouldUseFallback: true,
};

const useQueryMock = useQuery as unknown as {
  mockImplementation: (fn: (path: string, init?: unknown) => unknown) => void;
};

function renderPanel() {
  return render(
    <MemoryRouter>
      <PageContextProvider>
        <AppBarContext.Provider value={appBarValue}>
          <DAGDetailsPanel
            fileName="example"
            onClose={vi.fn()}
          />
        </AppBarContext.Provider>
      </PageContextProvider>
    </MemoryRouter>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('DAGDetailsPanel', () => {
  it('passes editor hints through to the dag list detail panel spec flow', () => {
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: {
            dag: { name: 'example-dag' },
            filePath: '/tmp/example.yaml',
            latestDAGRun: undefined,
            localDags: [],
            editorHints: {
              inheritedCustomStepTypes: [{ name: 'greet' }],
            },
          },
          error: undefined,
          mutate: vi.fn(),
        } as never;
      }

      return {
        data: undefined,
        error: undefined,
        mutate: vi.fn(),
      } as never;
    });

    renderPanel();

    expect(screen.getByText('Previewing example-dag [status]')).toBeInTheDocument();
    expect(screen.getByText('Inherited hints: 1')).toBeInTheDocument();
  });
});
