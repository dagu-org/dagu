// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import { useDAGRunFilterSuggestions } from '../useDAGRunFilterSuggestions';

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

function createFilters(
  overrides: Partial<
    Parameters<typeof useDAGRunFilterSuggestions>[0]['filters']
  > = {}
) {
  return {
    name: '',
    dagRunId: '',
    status: 'all',
    tags: [],
    fromDate: undefined,
    toDate: undefined,
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

async function flushMicrotasks() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('useDAGRunFilterSuggestions', () => {
  const formatDateForApi = vi.fn((value?: string) => {
    if (!value) {
      return undefined;
    }
    if (value === '2026-04-01T00:00') {
      return 100;
    }
    if (value === '2026-04-02T00:00') {
      return 200;
    }
    return 300;
  });

  beforeEach(() => {
    getMock.mockReset();
    formatDateForApi.mockClear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('builds suggestion queries from draft filters', async () => {
    getMock.mockResolvedValue({ data: { dagRuns: [] } });

    renderHook(() =>
      useDAGRunFilterSuggestions({
        field: 'name',
        filters: createFilters({
          name: 'payments',
          dagRunId: 'run-9',
          status: String(Status.Failed),
          tags: ['critical', 'prod'],
          fromDate: '2026-04-01T00:00',
          toDate: '2026-04-02T00:00',
        }),
        remoteNode: 'remote-a',
        isOpen: true,
        formatDateForApi,
      })
    );

    expect(getMock).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });

    await flushMicrotasks();

    expect(getMock).toHaveBeenCalledTimes(1);

    expect(getMock.mock.calls[0]?.[1]).toMatchObject({
      params: {
        query: {
          remoteNode: 'remote-a',
          name: 'payments',
          dagRunId: 'run-9',
          status: Status.Failed,
          tags: 'critical,prod',
          fromDate: 100,
          toDate: 200,
          limit: 500,
        },
      },
    });
  });

  it('fetches all pages so DAG name suggestions are not truncated and are sorted after dedupe', async () => {
    getMock
      .mockResolvedValueOnce({
        data: {
          dagRuns: [
            { name: 'zeta', dagRunId: 'run-3' },
            { name: 'Alpha', dagRunId: 'run-2' },
          ],
          nextCursor: 'cursor-1',
        },
      })
      .mockResolvedValueOnce({
        data: {
          dagRuns: [
            { name: 'beta', dagRunId: 'run-1' },
            { name: 'Alpha', dagRunId: 'run-0' },
          ],
        },
      });

    const { result } = renderHook(() =>
      useDAGRunFilterSuggestions({
        field: 'name',
        filters: createFilters({ name: 'a' }),
        remoteNode: 'local',
        isOpen: true,
        formatDateForApi,
      })
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });

    await flushMicrotasks();

    expect(result.current.suggestions).toEqual(['Alpha', 'beta', 'zeta']);

    expect(getMock).toHaveBeenCalledTimes(2);
    expect(getMock.mock.calls[1]?.[1]).toMatchObject({
      params: {
        query: expect.objectContaining({
          cursor: 'cursor-1',
          limit: 500,
        }),
      },
    });
  });

  it('dedupes run IDs while preserving newest-first order from the exact fetch', async () => {
    getMock.mockResolvedValue({
      data: {
        dagRuns: [
          { name: 'payments', dagRunId: 'run-9' },
          { name: 'payments', dagRunId: 'run-8' },
          { name: 'payments', dagRunId: 'run-9' },
          { name: 'payments', dagRunId: 'run-7' },
        ],
      },
    });

    const { result } = renderHook(() =>
      useDAGRunFilterSuggestions({
        field: 'dagRunId',
        filters: createFilters({ dagRunId: 'run' }),
        remoteNode: 'local',
        isOpen: true,
        formatDateForApi,
      })
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });

    await flushMicrotasks();

    expect(result.current.suggestions).toEqual(['run-9', 'run-8', 'run-7']);
  });

  it('debounces requests, aborts stale fetches, and clears results when the input is closed or empty', async () => {
    const firstRequest = createDeferred<{
      data: { dagRuns: Array<{ name: string; dagRunId: string }> };
    }>();
    const secondRequest = createDeferred<{
      data: { dagRuns: Array<{ name: string; dagRunId: string }> };
    }>();

    getMock
      .mockImplementationOnce(
        (_path: string, request?: { signal?: AbortSignal }) =>
          firstRequest.promise.then((value) => {
            if (request?.signal?.aborted) {
              throw new DOMException('Aborted', 'AbortError');
            }
            return value;
          })
      )
      .mockImplementationOnce(
        (_path: string, request?: { signal?: AbortSignal }) =>
          secondRequest.promise.then((value) => {
            if (request?.signal?.aborted) {
              throw new DOMException('Aborted', 'AbortError');
            }
            return value;
          })
      );

    const { result, rerender } = renderHook(
      ({ name, isOpen }: { name: string; isOpen: boolean }) =>
        useDAGRunFilterSuggestions({
          field: 'name',
          filters: createFilters({ name }),
          remoteNode: 'local',
          isOpen,
          formatDateForApi,
        }),
      {
        initialProps: { name: 'pay', isOpen: true },
      }
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(249);
    });
    expect(getMock).toHaveBeenCalledTimes(0);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });

    await flushMicrotasks();

    expect(getMock).toHaveBeenCalledTimes(1);

    const firstSignal = getMock.mock.calls[0]?.[1]?.signal as AbortSignal;

    rerender({ name: 'bill', isOpen: true });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });

    await flushMicrotasks();

    expect(getMock).toHaveBeenCalledTimes(2);

    expect(firstSignal.aborted).toBe(true);

    firstRequest.resolve({
      data: {
        dagRuns: [{ name: 'payments', dagRunId: 'run-1' }],
      },
    });
    secondRequest.resolve({
      data: {
        dagRuns: [{ name: 'billing', dagRunId: 'run-2' }],
      },
    });

    await flushMicrotasks();

    expect(result.current.suggestions).toEqual(['billing']);

    rerender({ name: 'bill', isOpen: false });

    await flushMicrotasks();

    expect(result.current.suggestions).toEqual([]);

    rerender({ name: '', isOpen: true });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });

    expect(getMock).toHaveBeenCalledTimes(2);
    expect(result.current.suggestions).toEqual([]);
  });
});
