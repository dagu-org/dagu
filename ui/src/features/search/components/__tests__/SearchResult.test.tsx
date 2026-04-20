// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SearchResult from '../SearchResult';
import { WorkspaceScope } from '@/api/v1/schema';

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}));

vi.mock('@/hooks/api', () => ({
  useClient: () => ({
    GET: getMock,
  }),
}));

describe('SearchResult', () => {
  beforeEach(() => {
    getMock.mockReset();
    getMock.mockResolvedValue({
      data: {
        matches: [],
        hasMore: false,
      },
    });
  });

  it('loads more DAG matches without sending a client-side limit override', async () => {
    render(
      <MemoryRouter>
        <SearchResult
          type="dag"
          query="needle"
          results={[
            {
              fileName: 'build',
              name: 'build',
              matches: [
                {
                  line: 'needle',
                  lineNumber: 3,
                  startLine: 3,
                },
              ],
              hasMoreMatches: true,
              nextMatchesCursor: 'cursor-1',
            },
          ]}
        />
      </MemoryRouter>
    );

    await userEvent.click(
      screen.getByRole('button', { name: 'Show more matches' })
    );

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(1);
    });

    expect(getMock).toHaveBeenCalledWith('/search/dags/{fileName}/matches', {
      params: {
        path: { fileName: 'build' },
        query: {
          remoteNode: 'local',
          workspaceScope: WorkspaceScope.accessible,
          q: 'needle',
          cursor: 'cursor-1',
        },
      },
    });
  });
});
