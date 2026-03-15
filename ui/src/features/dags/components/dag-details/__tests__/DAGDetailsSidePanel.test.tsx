// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import { useDAGSSE } from '@/hooks/useDAGSSE';
import DAGDetailsSidePanel from '../DAGDetailsSidePanel';

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
    forceEnqueue,
  }: {
    dag: { name: string };
    activeTab: string;
    forceEnqueue?: boolean;
  }) => (
    <div>
      Previewing {dag.name} [{activeTab}] {forceEnqueue ? 'forced' : 'default'}
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

const sseState = {
  data: null,
  error: null,
  isConnected: false,
  isConnecting: false,
  shouldUseFallback: true,
};
const useQueryMock = useQuery as unknown as {
  mockImplementation: (fn: (path: string) => unknown) => void;
};

function renderPanel(props?: Partial<React.ComponentProps<typeof DAGDetailsSidePanel>>) {
  return render(
    <MemoryRouter>
      <AppBarContext.Provider value={appBarValue}>
        <DAGDetailsSidePanel
          fileName="example"
          isOpen={true}
          onClose={vi.fn()}
          initialTab="status"
          {...props}
        />
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('DAGDetailsSidePanel', () => {
  it('shows a loading state while DAG details are pending', () => {
    vi.mocked(useDAGSSE).mockReturnValue(sseState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: undefined,
          error: undefined,
          mutate: vi.fn(),
        } as never;
      }

      return {
        data: undefined,
      } as never;
    });

    renderPanel();

    expect(screen.getByText('Loading DAG details...')).toBeInTheDocument();
  });

  it('shows a not-found state with a close action for 404s', () => {
    const onClose = vi.fn();
    vi.mocked(useDAGSSE).mockReturnValue(sseState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: undefined,
          error: { status: 404, message: 'not found' },
          mutate: vi.fn(),
        } as never;
      }

      return {
        data: undefined,
      } as never;
    });

    renderPanel({ onClose });

    expect(
      screen.getByText('DAG not found or has been deleted.')
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('shows an error state with retry when the load fails', () => {
    const mutate = vi.fn();
    vi.mocked(useDAGSSE).mockReturnValue(sseState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: undefined,
          error: { message: 'backend unavailable' },
          mutate,
        } as never;
      }

      return {
        data: undefined,
      } as never;
    });

    renderPanel();

    expect(screen.getByText('backend unavailable')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(mutate).toHaveBeenCalled();
  });

  it('renders DAG details content when data is available', () => {
    vi.mocked(useDAGSSE).mockReturnValue(sseState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: {
            dag: { name: 'example-dag' },
            filePath: '/tmp/example.yaml',
            latestDAGRun: undefined,
            localDags: [],
          },
          error: undefined,
          mutate: vi.fn(),
        } as never;
      }

      return {
        data: undefined,
      } as never;
    });

    renderPanel({ forceEnqueue: true, initialTab: 'history' });

    expect(
      screen.getByText('Previewing example-dag [history] forced')
    ).toBeInTheDocument();
  });
});
