// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import DAGActions from '../DAGActions';

const mocks = vi.hoisted(() => ({
  client: {
    POST: vi.fn(),
    GET: vi.fn(),
  },
  startModal: vi.fn(
    ({
      visible,
      onSubmit,
    }: {
      visible: boolean;
      onSubmit: (
        params: string,
        dagRunId?: string,
        immediate?: boolean
      ) => Promise<void>;
    }) =>
      visible ? (
        <div>
          <button type="button" onClick={() => void onSubmit('["x"]', 'manual-run')}>
            submit-enqueue
          </button>
          <button
            type="button"
            onClick={() => void onSubmit('["x"]', 'manual-run', true)}
          >
            submit-start
          </button>
        </div>
      ) : null
  ),
}));

vi.mock('../../dag-execution', () => ({
  StartDAGModal: (props: unknown) => mocks.startModal(props),
}));

vi.mock('../../../../../contexts/ConfigContext', () => ({
  useConfig: () => ({
    permissions: {
      runDags: true,
    },
  }),
}));

vi.mock('../../../../../contexts/WorkspaceContext', () => ({
  useOptionalWorkspace: () => ({
    selectedWorkspace: 'ops',
  }),
}));

vi.mock('../../../../../hooks/api', () => ({
  useClient: () => mocks.client,
}));

vi.mock('@/components/ui/error-modal', () => ({
  useErrorModal: () => ({
    showError: vi.fn(),
  }),
}));

vi.mock('@/components/ui/simple-toast', () => ({
  useSimpleToast: () => ({
    showToast: vi.fn(),
  }),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.client.POST.mockResolvedValue({ data: { dagRunId: 'queued-run' } });
  mocks.client.GET.mockResolvedValue({
    data: { dag: { name: 'example-dag' } },
    error: undefined,
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DAGActions', () => {
  it('shows cancel for failed runs with pending auto retries', () => {
    render(
      <DAGActions
        status={{
          name: 'retry-dag',
          dagRunId: 'run-1',
          status: Status.Failed,
          autoRetryCount: 1,
          autoRetryLimit: 3,
        }}
        fileName="retry-dag.yaml"
        dag={{ name: 'retry-dag' }}
        displayMode="full"
      />
    );

    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
  });

  it('disables retry for running DAG executions', () => {
    const view = render(
      <DAGActions
        status={{
          name: 'running-dag',
          dagRunId: 'run-1',
          status: Status.Running,
        }}
        fileName="running-dag.yaml"
        dag={{ name: 'running-dag' }}
        displayMode="full"
      />
    );

    const queries = within(view.container);

    expect(queries.getByRole('button', { name: 'Retry' })).toBeDisabled();
    expect(queries.getByRole('button', { name: 'Stop' })).toBeEnabled();
  });

  it('disables retry when there is no DAG run id', () => {
    const view = render(
      <DAGActions
        status={{
          name: 'finished-dag',
          status: Status.Failed,
          autoRetryCount: 3,
          autoRetryLimit: 3,
        }}
        fileName="finished-dag.yaml"
        dag={{ name: 'finished-dag' }}
        displayMode="full"
      />
    );

    expect(
      within(view.container).getByRole('button', { name: 'Retry' })
    ).toBeDisabled();
  });

  it('adds the selected workspace tag to enqueue requests', async () => {
    mocks.client.POST.mockResolvedValue({ data: { dagRunId: 'queued-run' } });

    render(
      <DAGActions
        fileName="example-dag.yaml"
        dag={{ name: 'example-dag' }}
        displayMode="full"
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Enqueue' }));
    fireEvent.click(await screen.findByRole('button', { name: 'submit-enqueue' }));

    expect(mocks.client.POST).toHaveBeenCalledWith('/dags/{fileName}/enqueue', {
      params: {
        path: { fileName: 'example-dag.yaml' },
        query: { remoteNode: 'local' },
      },
      body: {
        params: '["x"]',
        dagRunId: 'manual-run',
        tags: ['workspace=ops'],
      },
    });
  });

  it('adds the selected workspace tag to start requests', async () => {
    mocks.client.POST.mockResolvedValue({ data: { dagRunId: 'started-run' } });

    render(
      <DAGActions
        fileName="example-dag.yaml"
        dag={{ name: 'example-dag' }}
        displayMode="full"
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Enqueue' }));
    fireEvent.click(await screen.findByRole('button', { name: 'submit-start' }));

    expect(mocks.client.POST).toHaveBeenCalledWith('/dags/{fileName}/start', {
      params: {
        path: { fileName: 'example-dag.yaml' },
        query: { remoteNode: 'local' },
      },
      body: {
        params: '["x"]',
        dagRunId: 'manual-run',
        tags: ['workspace=ops'],
      },
    });
  });
});
