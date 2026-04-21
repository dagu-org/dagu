// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient, useQuery } from '@/hooks/api';
import { WorkspaceKind } from '@/lib/workspace';
import { DAGPicker } from '../DAGPicker';
import { DocPicker } from '../DocPicker';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

const appBarValue = {
  title: 'Agent',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
  workspaceSelection: {
    kind: WorkspaceKind.workspace,
    workspace: 'ops',
  },
};

const useQueryMock = useQuery as unknown as {
  mockImplementation: (fn: (path: string, params?: unknown) => unknown) => void;
};

const useClientMock = useClient as unknown as {
  mockReturnValue: (value: unknown) => void;
};

afterEach(() => {
  vi.clearAllMocks();
});

describe('agent context pickers', () => {
  it('scopes the DAG picker query by the selected workspace', () => {
    const queryCalls: Array<{ path: string; params?: unknown }> = [];
    useQueryMock.mockImplementation((path, params) => {
      queryCalls.push({ path, params });
      return { data: { dags: [] } } as never;
    });

    render(
      <AppBarContext.Provider value={appBarValue}>
        <DAGPicker selectedDags={[]} onChange={vi.fn()} />
      </AppBarContext.Provider>
    );

    expect(queryCalls).toEqual([
      {
        path: '/dags',
        params: expect.objectContaining({
          params: {
            query: {
              remoteNode: 'local',
              perPage: 100,
              workspace: 'ops',
            },
          },
        }),
      },
    ]);
  });

  it('scopes the doc picker query by the selected workspace', async () => {
    const get = vi.fn().mockResolvedValue({ data: { items: [] } });
    useClientMock.mockReturnValue({ GET: get });

    render(
      <AppBarContext.Provider value={appBarValue}>
        <DocPicker
          selectedDocs={[]}
          onSelect={vi.fn()}
          onRemove={vi.fn()}
          isOpen={true}
          onClose={vi.fn()}
          filterQuery=""
        />
      </AppBarContext.Provider>
    );

    await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
    expect(get).toHaveBeenCalledWith(
      '/docs',
      expect.objectContaining({
        params: {
          query: {
            remoteNode: 'local',
            flat: true,
            perPage: 200,
            workspace: 'ops',
          },
        },
      })
    );
  });
});
