// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { WorkspaceKind } from '@/lib/workspace';
import DAGRuns from '..';

vi.mock('@/contexts/SearchStateContext', () => ({
  useSearchState: () => ({
    readState: vi.fn(() => null),
    writeState: vi.fn(),
  }),
}));

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: {
      dagRunsViewMode: 'list',
    },
    updatePreference: vi.fn(),
  }),
}));

vi.mock('@/hooks/api', () => ({
  useQuery: () => ({
    data: { labels: [] },
  }),
}));

vi.mock('@/features/dag-runs/hooks/dagRunPagination', () => ({
  usePaginatedDAGRuns: () => ({
    dagRuns: [],
    isLoadingMore: false,
    loadMoreError: null,
    hasMore: false,
    refresh: vi.fn(),
    loadMore: vi.fn(),
  }),
}));

vi.mock('@/features/dag-runs/hooks/useBulkDAGRunSelection', () => ({
  useBulkDAGRunSelection: () => ({
    clearSelection: vi.fn(),
    replaceSelection: vi.fn(),
    selectAllLoaded: vi.fn(),
    selectedKeys: new Set(),
    selectedRuns: [],
    toggleSelection: vi.fn(),
  }),
}));

vi.mock('@/features/dag-runs/components/common/DAGRunBatchActions', () => ({
  default: () => null,
}));

vi.mock('@/features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: () => null,
}));

vi.mock(
  '@/features/dag-runs/components/dag-run-list/DAGRunGroupedView',
  () => ({
    default: () => <div>Grouped Runs</div>,
  })
);

vi.mock('@/features/dag-runs/components/dag-run-list/DAGRunTable', () => ({
  default: () => <div>Run Table</div>,
}));

const config = {
  tzOffsetInSec: undefined,
} as Config;

beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

function renderPage(setTitle = vi.fn()): void {
  render(
    <MemoryRouter initialEntries={['/dag-runs']}>
      <ConfigContext.Provider value={config}>
        <AppBarContext.Provider
          value={
            {
              setTitle,
              selectedRemoteNode: 'local',
              workspaceSelection: { kind: WorkspaceKind.all },
            } as never
          }
        >
          <DAGRuns />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('DAGRuns page', () => {
  it('uses the Executions page title', () => {
    const setTitle = vi.fn();

    renderPage(setTitle);

    expect(
      screen.getByRole('heading', { name: /^executions$/i })
    ).toBeVisible();
    expect(screen.queryByRole('heading', { name: /dag runs/i })).toBeNull();
    expect(setTitle).toHaveBeenCalledWith('Executions');
  });

  it('uses consistent filter control sizing', () => {
    renderPage();

    expect(
      screen.getByPlaceholderText('Filter by DAG name...').className
    ).toContain('h-9');
    expect(
      screen.getByPlaceholderText('Filter by Run ID...').className
    ).toContain('h-9');
    expect(screen.getByRole('combobox', { name: 'Status' }).className).toContain(
      'h-9'
    );
    expect(
      screen.getByRole('combobox', { name: 'Date preset' }).className
    ).toContain('h-9');
    expect(screen.getByRole('button', { name: 'Search' }).className).toContain(
      'h-9'
    );

    const labelInput = screen.getByRole('combobox', {
      name: 'Filter by labels...',
    });
    expect(labelInput.parentElement?.className).toContain('min-h-9');
    expect(labelInput.parentElement?.className).toContain('bg-card');

    expect(screen.getByRole('combobox', { name: 'Status' })).toHaveTextContent(
      'All Statuses'
    );
  });
});
