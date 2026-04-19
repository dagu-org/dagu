// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { useBulkDAGRunSelection } from '../useBulkDAGRunSelection';

const buildDagRun = (name: string, dagRunId: string) => ({
  name,
  dagRunId,
  status: Status.Failed,
  statusLabel: StatusLabel.failed,
  artifactsAvailable: false,
  autoRetryCount: 0,
  autoRetryLimit: 0,
  triggerType: TriggerType.manual,
  queuedAt: '2026-03-13T10:00:00Z',
  startedAt: '2026-03-13T10:01:00Z',
  finishedAt: '2026-03-13T10:02:00Z',
});

type HookDagRun = ReturnType<typeof buildDagRun>;

describe('useBulkDAGRunSelection', () => {
  it('toggles a single DAG run on and off', () => {
    const dagRuns = [
      buildDagRun('alpha', 'run-1'),
      buildDagRun('beta', 'run-2'),
    ];

    const { result } = renderHook(() => useBulkDAGRunSelection(dagRuns));

    act(() => {
      result.current.toggleSelection({ name: 'alpha', dagRunId: 'run-1' });
    });

    expect(result.current.selectedCount).toBe(1);
    expect(result.current.selectedRuns).toEqual([
      { name: 'alpha', dagRunId: 'run-1' },
    ]);

    act(() => {
      result.current.toggleSelection({ name: 'alpha', dagRunId: 'run-1' });
    });

    expect(result.current.selectedCount).toBe(0);
    expect(result.current.selectedRuns).toEqual([]);
  });

  it('selects all currently visible DAG runs', () => {
    const dagRuns = [
      buildDagRun('alpha', 'run-1'),
      buildDagRun('beta', 'run-2'),
    ];

    const { result } = renderHook(() => useBulkDAGRunSelection(dagRuns));

    act(() => {
      result.current.selectAllLoaded();
    });

    expect(result.current.selectedCount).toBe(2);
    expect(result.current.selectedRuns).toEqual([
      { name: 'alpha', dagRunId: 'run-1' },
      { name: 'beta', dagRunId: 'run-2' },
    ]);
  });

  it('prunes selections that disappear after the visible dataset refreshes', () => {
    const initialRuns = [
      buildDagRun('alpha', 'run-1'),
      buildDagRun('beta', 'run-2'),
    ];
    const remainingRun = initialRuns[1] as HookDagRun;
    const { result, rerender } = renderHook(
      ({ dagRuns }: { dagRuns: HookDagRun[] }) =>
        useBulkDAGRunSelection(dagRuns),
      {
        initialProps: {
          dagRuns: initialRuns,
        },
      }
    );

    act(() => {
      result.current.selectAllLoaded();
    });

    rerender({
      dagRuns: [remainingRun],
    });

    expect(result.current.selectedCount).toBe(1);
    expect(result.current.selectedRuns).toEqual([
      { name: 'beta', dagRunId: 'run-2' },
    ]);
  });

  it('replaces the selection with the provided visible items', () => {
    const dagRuns = [
      buildDagRun('alpha', 'run-1'),
      buildDagRun('beta', 'run-2'),
    ];

    const { result } = renderHook(() => useBulkDAGRunSelection(dagRuns));

    act(() => {
      result.current.replaceSelection([{ name: 'beta', dagRunId: 'run-2' }]);
    });

    expect(result.current.selectedCount).toBe(1);
    expect(result.current.selectedRuns).toEqual([
      { name: 'beta', dagRunId: 'run-2' },
    ]);
  });
});
