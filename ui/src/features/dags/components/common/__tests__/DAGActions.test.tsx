import { render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import DAGActions from '../DAGActions';

vi.mock('../../dag-execution', () => ({
  StartDAGModal: () => null,
}));

vi.mock('../../../../../contexts/ConfigContext', () => ({
  useConfig: () => ({
    permissions: {
      runDags: true,
    },
  }),
}));

vi.mock('../../../../../hooks/api', () => ({
  useClient: () => ({
    POST: vi.fn(),
    GET: vi.fn(),
  }),
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
});
