import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import { useLiveConnection } from '@/hooks/useAppLive';
import DAGRunDetailsPanel from '../DAGRunDetailsPanel';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/hooks/useAppLive', () => ({
  liveFallbackOptions: vi.fn(() => ({})),
  useLiveConnection: vi.fn(),
  useLiveDAGRuns: vi.fn(),
}));

vi.mock('../DAGRunDetailsContent', () => ({
  default: ({ dagRun }: { dagRun: { dagRunId: string } }) => (
    <div>run {dagRun.dagRunId}</div>
  ),
}));

const appBarValue = {
  title: 'Runs',
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
  mockImplementation: (
    fn: (path: string, init?: unknown) => unknown
  ) => void;
};

function renderPanel() {
  return render(
    <MemoryRouter>
      <AppBarContext.Provider value={appBarValue}>
        <DAGRunDetailsPanel
          name="child-dag"
          dagRunId="child-run"
          onClose={vi.fn()}
        />
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

afterEach(() => {
  vi.clearAllMocks();
  window.history.pushState({}, '', '/');
});

describe('DAGRunDetailsPanel', () => {
  it('enables the regular dag-run query and disables the sub-dag query by null init', () => {
    const queryCalls: Array<{ path: string; init?: unknown }> = [];
    vi.mocked(useLiveConnection).mockReturnValue(liveState);
    useQueryMock.mockImplementation((path, init) => {
      queryCalls.push({ path, init });
      if (path === '/dag-runs/{name}/{dagRunId}') {
        return {
          data: { dagRunDetails: { dagRunId: 'child-run' } },
          mutate: vi.fn(),
        } as never;
      }
      return {
        data: undefined,
        mutate: vi.fn(),
      } as never;
    });

    renderPanel();

    expect(
      queryCalls.find((call) => call.path === '/dag-runs/{name}/{dagRunId}')?.init
    ).toEqual(
      expect.objectContaining({
        params: expect.objectContaining({
          path: { name: 'child-dag', dagRunId: 'child-run' },
        }),
      })
    );
    expect(
      queryCalls.find(
        (call) => call.path === '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}'
      )?.init
    ).toBeNull();
    expect(screen.getByText('run child-run')).toBeInTheDocument();
  });

  it('enables the sub-dag query and disables the regular dag-run query by null init', () => {
    const queryCalls: Array<{ path: string; init?: unknown }> = [];
    window.history.pushState(
      {},
      '',
      '/?subDAGRunId=sub-run&dagRunId=root-run&dagRunName=root-dag'
    );
    vi.mocked(useLiveConnection).mockReturnValue(liveState);
    useQueryMock.mockImplementation((path, init) => {
      queryCalls.push({ path, init });
      if (path === '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}') {
        return {
          data: { dagRunDetails: { dagRunId: 'sub-run' } },
          mutate: vi.fn(),
        } as never;
      }
      return {
        data: undefined,
        mutate: vi.fn(),
      } as never;
    });

    renderPanel();

    expect(
      queryCalls.find(
        (call) => call.path === '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}'
      )?.init
    ).toEqual(
      expect.objectContaining({
        params: expect.objectContaining({
          path: {
            name: 'root-dag',
            dagRunId: 'root-run',
            subDAGRunId: 'sub-run',
          },
        }),
      })
    );
    expect(
      queryCalls.find((call) => call.path === '/dag-runs/{name}/{dagRunId}')?.init
    ).toBeNull();
    expect(screen.getByText('run sub-run')).toBeInTheDocument();
  });
});
