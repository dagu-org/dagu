// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useQuery } from '@/hooks/api';
import { useDAGRunActivitySSE } from '@/hooks/useDAGRunActivitySSE';
import { useDAGRunActivity } from '../useDAGRunActivity';

vi.mock('@/hooks/api', () => ({ useQuery: vi.fn() }));
vi.mock('@/hooks/useDAGRunActivitySSE', () => ({
  useDAGRunActivitySSE: vi.fn(),
}));

const useQueryMock = vi.mocked(
  useQuery as unknown as (
    path: string,
    init: unknown,
    config: Record<string, unknown>
  ) => unknown
);
const useSSEMock = vi.mocked(useDAGRunActivitySSE);

type QueryResult = { data?: unknown; error?: unknown };
let queryResult: QueryResult = {};

function setVisibility(state: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    get: () => state,
  });
  document.dispatchEvent(new Event('visibilitychange'));
}

function lastSSEEnabled(): boolean | undefined {
  const calls = useSSEMock.mock.calls;
  return calls[calls.length - 1]?.[1];
}

function lastQueryConfig(): Record<string, unknown> | undefined {
  const calls = useQueryMock.mock.calls;
  return calls[calls.length - 1]?.[2];
}

describe('useDAGRunActivity', () => {
  beforeEach(() => {
    queryResult = {};
    setVisibility('visible');
    useSSEMock.mockReturnValue({
      data: null,
      error: null,
      isConnected: true,
      isConnecting: false,
      shouldUseFallback: false,
    } as never);
    useQueryMock.mockImplementation(() => ({
      ...queryResult,
      isLoading: false,
      mutate: vi.fn().mockResolvedValue(undefined),
    }));
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('streams a live window only while the tab is visible', () => {
    renderHook(() => useDAGRunActivity({ kind: 'live' }, {}, 'local'));
    expect(lastSSEEnabled()).toBe(true);

    act(() => setVisibility('hidden'));
    expect(lastSSEEnabled()).toBe(false);

    act(() => setVisibility('visible'));
    expect(lastSSEEnabled()).toBe(true);
  });

  it('reads a fixed window once without streaming', () => {
    renderHook(() =>
      useDAGRunActivity({ kind: 'fixed', from: 1, to: 2 }, {}, 'local')
    );

    expect(lastSSEEnabled()).toBe(false);
    expect(lastQueryConfig()).toMatchObject({ refreshInterval: 0 });
  });

  // An older remote node answers 404; polling it forever would only repeat
  // the error.
  it('stops streaming and polling a node that fails the read until refreshed', () => {
    queryResult = { error: { message: 'not found' } };
    const { result } = renderHook(() =>
      useDAGRunActivity({ kind: 'live' }, {}, 'old-node')
    );

    expect(result.current.unavailable).toBe(true);
    expect(lastSSEEnabled()).toBe(false);
    expect(lastQueryConfig()).toMatchObject({ refreshInterval: 0 });

    queryResult = { data: { workflows: [] } };
    act(() => {
      void result.current.refresh();
    });
    expect(result.current.unavailable).toBe(false);
    expect(lastSSEEnabled()).toBe(true);
  });
});
