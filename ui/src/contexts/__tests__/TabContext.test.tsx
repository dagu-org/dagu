// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, cleanup, renderHook } from '@testing-library/react';
import React from 'react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { TabProvider, useTabContext } from '../TabContext';

function wrapperFor(storageKey: string) {
  return ({ children }: { children: React.ReactNode }) => (
    <TabProvider storageKey={storageKey}>{children}</TabProvider>
  );
}

describe('TabProvider', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it('restores DAG tabs only from the active workspace', () => {
    const scopeA = 'dagu_dag_tabs:{"remoteNode":"local","workspace":"ops"}';
    const scopeB =
      'dagu_dag_tabs:{"remoteNode":"local","workspace":"platform"}';

    const first = renderHook(() => useTabContext(), {
      wrapper: wrapperFor(scopeA),
    });

    act(() => {
      first.result.current.addTab('alpha.yaml', 'Alpha');
    });

    expect(first.result.current.getActiveFileName()).toBe('alpha.yaml');

    first.unmount();

    const second = renderHook(() => useTabContext(), {
      wrapper: wrapperFor(scopeB),
    });

    expect(second.result.current.tabs).toHaveLength(0);
    expect(second.result.current.getActiveFileName()).toBeNull();

    second.unmount();

    const restored = renderHook(() => useTabContext(), {
      wrapper: wrapperFor(scopeA),
    });

    expect(restored.result.current.tabs).toHaveLength(1);
    expect(restored.result.current.getActiveFileName()).toBe('alpha.yaml');
  });

  it('does not load legacy unscoped tabs into a scoped provider', () => {
    localStorage.setItem(
      'dagu_dag_tabs',
      JSON.stringify({
        tabs: [{ id: 'legacy', fileName: 'legacy.yaml', title: 'Legacy' }],
        activeTabId: 'legacy',
      })
    );

    const { result } = renderHook(() => useTabContext(), {
      wrapper: wrapperFor(
        'dagu_dag_tabs:{"remoteNode":"local","workspace":"ops"}'
      ),
    });

    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.getActiveFileName()).toBeNull();
  });
});
