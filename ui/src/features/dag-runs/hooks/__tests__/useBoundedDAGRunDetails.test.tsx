import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useBoundedDAGRunDetails } from '../useBoundedDAGRunDetails';

const { fetchDAGRunDetailsMock, dagRunSSEState, subDAGRunSSEState } =
  vi.hoisted(() => ({
    fetchDAGRunDetailsMock: vi.fn(),
    dagRunSSEState: {
      current: {
        data: null,
        error: null,
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
      } as any,
    },
    subDAGRunSSEState: {
      current: {
        data: null,
        error: null,
        isConnected: false,
        isConnecting: false,
        shouldUseFallback: true,
      } as any,
    },
  }));

vi.mock('@/hooks/useDAGRunSSE', () => ({
  useDAGRunSSE: vi.fn(() => dagRunSSEState.current),
}));

vi.mock('@/hooks/useSubDAGRunSSE', () => ({
  useSubDAGRunSSE: vi.fn(() => subDAGRunSSEState.current),
}));

vi.mock('../dagRunDetailsRequest', () => ({
  fetchDAGRunDetails: fetchDAGRunDetailsMock,
  matchesRequestedDAGRunDetails: (
    details: { dagRunId?: string } | null | undefined,
    requestedDagRunId: string
  ) => {
    if (!details) {
      return false;
    }
    return (
      requestedDagRunId === 'latest' || details.dagRunId === requestedDagRunId
    );
  },
}));

function createTarget(overrides: Record<string, string> = {}) {
  return {
    remoteNode: 'local',
    name: 'billing',
    dagRunId: 'run-1',
    ...overrides,
  };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe('useBoundedDAGRunDetails', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    fetchDAGRunDetailsMock.mockReset();
    dagRunSSEState.current = {
      data: null,
      error: null,
      isConnected: true,
      isConnecting: false,
      shouldUseFallback: false,
    };
    subDAGRunSSEState.current = {
      data: null,
      error: null,
      isConnected: false,
      isConnecting: false,
      shouldUseFallback: true,
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not keep polling while the DAG-run SSE topic is healthy', async () => {
    fetchDAGRunDetailsMock.mockResolvedValue({ dagRunId: 'run-1' });

    renderHook(() =>
      useBoundedDAGRunDetails({
        target: createTarget(),
        pollIntervalMs: 2000,
      })
    );

    await act(async () => {
      await Promise.resolve();
    });
    expect(fetchDAGRunDetailsMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(6000);
    });

    expect(fetchDAGRunDetailsMock).toHaveBeenCalledTimes(1);
  });

  it('hydrates from SSE payloads and aborts the in-flight fallback request', async () => {
    const deferred = createDeferred<{ dagRunId: string }>();
    let capturedSignal: AbortSignal | undefined;
    fetchDAGRunDetailsMock.mockImplementation(
      (_target: unknown, init?: { signal?: AbortSignal }) => {
        capturedSignal = init?.signal;
        return deferred.promise;
      }
    );

    const { result, rerender } = renderHook(() =>
      useBoundedDAGRunDetails({
        target: createTarget(),
        pollIntervalMs: 2000,
      })
    );

    await act(async () => {
      await Promise.resolve();
    });
    expect(fetchDAGRunDetailsMock).toHaveBeenCalledTimes(1);

    act(() => {
      dagRunSSEState.current = {
        data: {
          dagRunDetails: {
            dagRunId: 'run-1',
            name: 'billing',
          },
        },
        error: null,
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
      };
      rerender();
    });

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.data).toMatchObject({
      dagRunId: 'run-1',
      name: 'billing',
    });

    expect(capturedSignal?.aborted).toBe(true);
  });
});
