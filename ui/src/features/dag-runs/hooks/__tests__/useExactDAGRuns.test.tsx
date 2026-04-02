import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useExactDAGRuns, type DAGRunListQuery } from '../dagRunPagination';

const getMock = vi.fn();
const client = {
  GET: getMock,
};

vi.mock('@/hooks/api', () => ({
  useClient: () => client,
  useQuery: vi.fn(),
}));

vi.mock('@/hooks/useAppLive', () => ({
  liveFallbackOptions: vi.fn(),
  useLiveConnection: vi.fn(() => ({
    isConnected: false,
    isConnecting: false,
    shouldUseFallback: true,
    error: null,
  })),
  useLiveDAGRuns: vi.fn(),
  useLiveInvalidation: vi.fn(),
}));

function createQuery(overrides: Partial<DAGRunListQuery> = {}): DAGRunListQuery {
  return {
    remoteNode: 'local',
    fromDate: 100,
    ...overrides,
  };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('useExactDAGRuns', () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not refetch when rerendered with an equivalent inline query object', async () => {
    getMock.mockResolvedValue({ data: { dagRuns: [] } });

    const { rerender } = renderHook(({ query }) => useExactDAGRuns({ query }), {
      initialProps: { query: createQuery() },
    });

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(1);
    });

    rerender({ query: createQuery() });

    await act(async () => {
      await Promise.resolve();
    });

    expect(getMock).toHaveBeenCalledTimes(1);
  });

  it('refetches exactly once when the semantic query changes', async () => {
    getMock.mockResolvedValue({ data: { dagRuns: [] } });

    const { rerender } = renderHook(({ query }) => useExactDAGRuns({ query }), {
      initialProps: { query: createQuery() },
    });

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(1);
    });

    rerender({ query: createQuery({ fromDate: 200, name: 'billing' }) });

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(2);
    });

    expect(getMock.mock.calls[1]?.[1]).toMatchObject({
      params: {
        query: expect.objectContaining({
          fromDate: 200,
          name: 'billing',
          remoteNode: 'local',
        }),
      },
    });
  });

  it('refresh uses the latest query after rerender instead of a stale closure', async () => {
    getMock.mockResolvedValue({ data: { dagRuns: [] } });

    const { result, rerender } = renderHook(
      ({ query }) => useExactDAGRuns({ query }),
      {
        initialProps: { query: createQuery({ fromDate: 100 }) },
      }
    );

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(1);
    });

    rerender({ query: createQuery({ fromDate: 300, name: 'ops' }) });

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(2);
    });

    await act(async () => {
      await result.current.refresh();
    });

    expect(getMock).toHaveBeenCalledTimes(3);
    expect(getMock.mock.calls[2]?.[1]).toMatchObject({
      params: {
        query: expect.objectContaining({
          fromDate: 300,
          name: 'ops',
          remoteNode: 'local',
        }),
      },
    });
  });

  it('ignores stale responses after a query change and aborts the older request', async () => {
    const firstRequest = createDeferred<{ data: { dagRuns: Array<{ name: string; dagRunId: string }> } }>();
    const secondRequest = createDeferred<{ data: { dagRuns: Array<{ name: string; dagRunId: string }> } }>();

    getMock
      .mockImplementationOnce(
        (_path: string, request?: { signal?: AbortSignal }) => {
          return firstRequest.promise.then((value) => {
            if (request?.signal?.aborted) {
              throw new DOMException('Aborted', 'AbortError');
            }
            return value;
          });
        }
      )
      .mockImplementationOnce(
        (_path: string, request?: { signal?: AbortSignal }) => {
          return secondRequest.promise.then((value) => {
            if (request?.signal?.aborted) {
              throw new DOMException('Aborted', 'AbortError');
            }
            return value;
          });
        }
      );

    const { result, rerender } = renderHook(
      ({ query }) => useExactDAGRuns({ query }),
      {
        initialProps: { query: createQuery({ fromDate: 100 }) },
      }
    );

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(1);
    });

    const firstSignal = getMock.mock.calls[0]?.[1]?.signal as AbortSignal;

    rerender({ query: createQuery({ fromDate: 400, name: 'new-dag' }) });

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(2);
    });

    expect(firstSignal.aborted).toBe(true);

    firstRequest.resolve({
      data: {
        dagRuns: [{ name: 'stale', dagRunId: 'old' }],
      },
    });
    secondRequest.resolve({
      data: {
        dagRuns: [{ name: 'fresh', dagRunId: 'new' }],
      },
    });

    await waitFor(() => {
      expect(result.current.data).toEqual([{ name: 'fresh', dagRunId: 'new' }]);
    });
  });

  it('polls only on the configured fallback interval instead of every rerender', async () => {
    vi.useFakeTimers();
    getMock.mockResolvedValue({ data: { dagRuns: [] } });

    renderHook(() =>
      useExactDAGRuns({
        query: createQuery(),
        fallbackIntervalMs: 2000,
      })
    );

    await act(async () => {
      await Promise.resolve();
    });
    expect(getMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999);
    });
    expect(getMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(getMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(getMock).toHaveBeenCalledTimes(3);
  });
});
