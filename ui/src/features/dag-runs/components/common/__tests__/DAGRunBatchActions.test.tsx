// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import DAGRunBatchActions from '../DAGRunBatchActions';

const postMock = vi.fn();
const showErrorMock = vi.fn();
const showToastMock = vi.fn();

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
}));

vi.mock('@/components/ui/error-modal', () => ({
  useErrorModal: () => ({
    showError: showErrorMock,
  }),
}));

vi.mock('@/components/ui/simple-toast', () => ({
  useSimpleToast: () => ({
    showToast: showToastMock,
  }),
}));

const useClientMock = vi.mocked(useClient);
const appBarContextValue = {
  title: '',
  setTitle: vi.fn(),
  remoteNodes: [],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

describe('DAGRunBatchActions', () => {
  beforeEach(() => {
    postMock.mockReset();
    showErrorMock.mockReset();
    showToastMock.mockReset();
    useClientMock.mockReturnValue({
      POST: postMock,
    } as never);
  });

  it('renders mixed batch results in the response order', async () => {
    postMock.mockResolvedValue({
      data: {
        totalCount: 2,
        successCount: 1,
        failureCount: 1,
        results: [
          {
            name: 'beta',
            dagRunId: 'run-2',
            ok: false,
            error: 'unable to reschedule',
          },
          {
            name: 'alpha',
            dagRunId: 'run-1',
            ok: true,
            newDagRunId: 'run-1-copy',
            queued: false,
          },
        ],
      },
    });
    const onActionComplete = vi.fn().mockResolvedValue(undefined);
    const onClearSelection = vi.fn();

    render(
      <AppBarContext.Provider value={appBarContextValue}>
        <DAGRunBatchActions
          selectedRuns={[
            { name: 'alpha', dagRunId: 'run-1' },
            { name: 'beta', dagRunId: 'run-2' },
          ]}
          visibleCount={2}
          onSelectAllVisible={vi.fn()}
          onClearSelection={onClearSelection}
          onActionComplete={onActionComplete}
        />
      </AppBarContext.Provider>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Reschedule selected' }));
    fireEvent.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Reschedule selected',
      })
    );

    expect(await screen.findByText('unable to reschedule')).toBeInTheDocument();
    const items = await screen.findAllByTestId('batch-action-result-item');

    expect(postMock).toHaveBeenCalledWith('/dag-runs/reschedule-batch', {
      params: {
        query: {
          remoteNode: 'local',
        },
      },
      body: {
        items: [
          { name: 'alpha', dagRunId: 'run-1' },
          { name: 'beta', dagRunId: 'run-2' },
        ],
      },
    });
    expect(onActionComplete).toHaveBeenCalled();
    expect(onClearSelection).toHaveBeenCalled();
    expect(showToastMock).not.toHaveBeenCalled();
    expect(items[0]).toHaveTextContent('beta');
    expect(items[0]).toHaveTextContent('run-2');
    expect(items[1]).toHaveTextContent('alpha');
    expect(items[1]).toHaveTextContent('run-1-copy');
  });
});
