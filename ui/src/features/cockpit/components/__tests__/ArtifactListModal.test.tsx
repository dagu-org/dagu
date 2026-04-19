// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { ArtifactListModal } from '../ArtifactListModal';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
}));

const appBarValue = {
  title: 'Cockpit',
  setTitle: vi.fn(),
  remoteNodes: ['local', 'edge'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'edge',
  selectRemoteNode: vi.fn(),
};

const run = {
  dagRunId: 'run-1',
  name: 'artifact-dag',
  status: Status.Success,
  statusLabel: StatusLabel.succeeded,
  artifactsAvailable: true,
  autoRetryCount: 0,
  triggerType: TriggerType.manual,
  queuedAt: '',
  scheduleTime: '',
  startedAt: '2026-03-16T00:00:00Z',
  finishedAt: '2026-03-16T00:01:00Z',
};

afterEach(() => {
  vi.clearAllMocks();
});

describe('ArtifactListModal', () => {
  it('loads and renders the run artifact tree for the selected remote node', async () => {
    const get = vi.fn(async () => ({
      data: {
        items: [
          {
            type: 'directory',
            name: 'reports',
            path: 'reports',
            children: [
              {
                type: 'file',
                name: 'summary.md',
                path: 'reports/summary.md',
                size: 12,
              },
            ],
          },
        ],
      },
      error: undefined,
    }));

    vi.mocked(useClient).mockReturnValue({ GET: get } as never);

    render(
      <AppBarContext.Provider value={appBarValue}>
        <ArtifactListModal run={run} isOpen={true} onClose={() => {}} />
      </AppBarContext.Provider>
    );

    expect((await screen.findAllByText('reports')).length).toBeGreaterThan(0);
    expect(screen.getByText('summary.md')).toBeInTheDocument();
    expect(screen.getByText('12 B')).toBeInTheDocument();

    await waitFor(() => {
      expect(get).toHaveBeenCalledWith(
        '/dag-runs/{name}/{dagRunId}/artifacts',
        {
          params: {
            path: { name: 'artifact-dag', dagRunId: 'run-1' },
            query: { remoteNode: 'edge', recursive: true },
          },
          signal: expect.any(AbortSignal),
        }
      );
    });
  });
});
