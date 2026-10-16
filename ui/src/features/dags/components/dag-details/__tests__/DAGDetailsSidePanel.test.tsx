// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';
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
    dagRunId,
    editorHints,
    forceEnqueue,
    onEnqueue,
  }: {
    dag: { name: string };
    activeTab: string;
    dagRunId?: string;
    editorHints?: { inheritedLegacyDefinitions?: unknown[] };
    forceEnqueue?: boolean;
    onEnqueue?: (
      params: string,
      dagRunId?: string,
      immediate?: boolean
    ) => string | void | Promise<string | void>;
  }) => {
    const [dialogOpen, setDialogOpen] = React.useState(false);
    return (
      <div>
        <div>
          Previewing {dag.name} [{activeTab}]{' '}
          {forceEnqueue ? 'forced' : 'default'} {dagRunId || 'latest'}
        </div>
        <div>
          Inherited hints:{' '}
          {editorHints?.inheritedLegacyDefinitions?.length ?? 0}
        </div>
        {onEnqueue ? (
          <button
            type="button"
            onClick={async () => {
              await onEnqueue('["x"]', 'manual-run');
            }}
          >
            Enqueue Now
          </button>
        ) : null}
        <button type="button" onClick={() => setDialogOpen(true)}>
          Open dialog
        </button>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent>
            <DialogTitle>Nested dialog</DialogTitle>
            <DialogDescription>
              Opened from the panel content.
            </DialogDescription>
          </DialogContent>
        </Dialog>
      </div>
    );
  },
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

function mockLoadedDAG() {
  vi.mocked(useDAGSSE).mockReturnValue(liveState);
  useQueryMock.mockImplementation((path) => {
    if (path === '/dags/{fileName}') {
      return {
        data: {
          dag: { name: 'example-dag' },
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
}

function renderPanel(
  props?: Partial<React.ComponentProps<typeof DAGDetailsSidePanel>>
) {
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
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
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

    expect(screen.getByText('Loading workflow...')).toBeInTheDocument();
  });

  it('uses a null query key while closed so the detail request is truly disabled', () => {
    const queryCalls: Array<{ path: string; init?: unknown }> = [];
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
    useQueryMock.mockImplementation((path, init) => {
      queryCalls.push({ path, init });
      return {
        data: undefined,
        error: undefined,
        mutate: vi.fn(),
      } as never;
    });

    renderPanel({ isOpen: false });

    expect(
      queryCalls.find((call) => call.path === '/dags/{fileName}')?.init
    ).toBeNull();
  });

  it('shows a not-found state with a close action for 404s', () => {
    const onClose = vi.fn();
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
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

    // Both the toolbar and the not-found state offer a close action.
    const closeButtons = screen.getAllByRole('button', { name: 'Close' });
    expect(closeButtons).toHaveLength(2);
    fireEvent.click(closeButtons[closeButtons.length - 1]!);
    expect(onClose).toHaveBeenCalled();
  });

  it('shows an error state with retry when the load fails', () => {
    const mutate = vi.fn();
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
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
    mockLoadedDAG();

    renderPanel({ forceEnqueue: true, initialTab: 'history' });

    expect(
      screen.getByText('Previewing example-dag [history] forced latest')
    ).toBeInTheDocument();
  });

  it('passes editor hints through to the modal DAG spec flow', () => {
    vi.mocked(useDAGSSE).mockReturnValue(liveState);
    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/{fileName}') {
        return {
          data: {
            dag: { name: 'example-dag' },
            latestDAGRun: undefined,
            localDags: [],
            editorHints: {
              inheritedLegacyDefinitions: [{ name: 'greet' }],
            },
          },
          error: undefined,
          mutate: vi.fn(),
        } as never;
      }

      return {
        data: undefined,
      } as never;
    });

    renderPanel();

    expect(screen.getByText('Inherited hints: 1')).toBeInTheDocument();
  });

  it('closes on Escape', () => {
    const onClose = vi.fn();
    mockLoadedDAG();

    renderPanel({ onClose });
    fireEvent.keyDown(document.body, { key: 'Escape' });

    expect(onClose).toHaveBeenCalled();
  });

  it('stays open when Escape closes a dialog inside it', () => {
    const onClose = vi.fn();
    mockLoadedDAG();

    renderPanel({ onClose });
    fireEvent.click(screen.getByRole('button', { name: 'Open dialog' }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.keyDown(document.activeElement ?? document.body, {
      key: 'Escape',
    });

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});
