// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import DAGRunBatchActions from '../DAGRunBatchActions';

const postMock = vi.fn();

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
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

const createDeferred = <T,>() => {
  let resolve: (value: T) => void = () => undefined;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
};

describe('DAGRunBatchActions', () => {
  beforeEach(() => {
    postMock.mockReset();
    useClientMock.mockReturnValue({
      POST: postMock,
    } as never);
  });

  it('submits reschedules sequentially and keeps failed items selected', async () => {
    const firstRequest = createDeferred<{
      data: {
        dagRunId: string;
        queued: boolean;
      };
    }>();

    postMock
      .mockImplementationOnce(() => firstRequest.promise)
      .mockResolvedValueOnce({
        error: {
          message: 'unable to reschedule',
        },
      });

    const onActionComplete = vi.fn().mockResolvedValue(undefined);
    const onReplaceSelection = vi.fn();

    render(
      <AppBarContext.Provider value={appBarContextValue}>
        <DAGRunBatchActions
          selectedRuns={[
            { name: 'alpha', dagRunId: 'run-1' },
            { name: 'beta', dagRunId: 'run-2' },
          ]}
          matchingCount={2}
          onSelectAllMatching={vi.fn()}
          onClearSelection={vi.fn()}
          onReplaceSelection={onReplaceSelection}
          onActionComplete={onActionComplete}
        />
      </AppBarContext.Provider>
    );

    fireEvent.click(
      screen.getByRole('button', { name: 'Reschedule selected' })
    );
    fireEvent.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Reschedule 2 Runs',
      })
    );

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));
    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/dag-runs/{name}/{dagRunId}/reschedule',
      {
        params: {
          path: {
            name: 'alpha',
            dagRunId: 'run-1',
          },
          query: {
            remoteNode: 'local',
          },
        },
        body: {
          dagRunId: undefined,
        },
      }
    );

    expect(screen.getByText('Submitting requests...')).toBeInTheDocument();
    firstRequest.resolve({
      data: {
        dagRunId: 'run-1-copy',
        queued: false,
      },
    });

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      '/dag-runs/{name}/{dagRunId}/reschedule',
      {
        params: {
          path: {
            name: 'beta',
            dagRunId: 'run-2',
          },
          query: {
            remoteNode: 'local',
          },
        },
        body: {
          dagRunId: undefined,
        },
      }
    );

    expect(await screen.findByText('unable to reschedule')).toBeInTheDocument();
    const items = await screen.findAllByTestId('batch-action-result-item');

    expect(onActionComplete).toHaveBeenCalledTimes(1);
    expect(onReplaceSelection).toHaveBeenCalledWith([
      { name: 'beta', dagRunId: 'run-2' },
    ]);
    expect(items[0]).toHaveTextContent('alpha');
    expect(items[0]).toHaveTextContent('run-1-copy');
    expect(items[1]).toHaveTextContent('beta');
    expect(items[1]).toHaveTextContent('unable to reschedule');
  });

  it('uses the existing retry endpoint and shows submission-only success details', async () => {
    postMock.mockResolvedValueOnce({});
    const onReplaceSelection = vi.fn();

    render(
      <AppBarContext.Provider value={appBarContextValue}>
        <DAGRunBatchActions
          selectedRuns={[{ name: 'alpha', dagRunId: 'run-1' }]}
          matchingCount={1}
          onSelectAllMatching={vi.fn()}
          onClearSelection={vi.fn()}
          onReplaceSelection={onReplaceSelection}
          onActionComplete={vi.fn().mockResolvedValue(undefined)}
        />
      </AppBarContext.Provider>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Retry selected' }));
    fireEvent.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Retry 1 Run',
      })
    );

    expect(
      await screen.findByText('Retry request accepted')
    ).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith('/dag-runs/{name}/{dagRunId}/retry', {
      params: {
        path: {
          name: 'alpha',
          dagRunId: 'run-1',
        },
        query: {
          remoteNode: 'local',
        },
      },
      body: {
        dagRunId: 'run-1',
      },
    });
    expect(onReplaceSelection).toHaveBeenCalledWith([]);
  });

  it('treats a reschedule response without a new run ID as a failure', async () => {
    postMock.mockResolvedValueOnce({
      data: {},
    });

    render(
      <AppBarContext.Provider value={appBarContextValue}>
        <DAGRunBatchActions
          selectedRuns={[{ name: 'alpha', dagRunId: 'run-1' }]}
          matchingCount={1}
          onSelectAllMatching={vi.fn()}
          onClearSelection={vi.fn()}
          onReplaceSelection={vi.fn()}
          onActionComplete={vi.fn().mockResolvedValue(undefined)}
        />
      </AppBarContext.Provider>
    );

    fireEvent.click(
      screen.getByRole('button', { name: 'Reschedule selected' })
    );
    fireEvent.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Reschedule 1 Run',
      })
    );

    expect(
      await screen.findByText(
        'Reschedule request did not return a new DAG run ID.'
      )
    ).toBeInTheDocument();
  });
});
