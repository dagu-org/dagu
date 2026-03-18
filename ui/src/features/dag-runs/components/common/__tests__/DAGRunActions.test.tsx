import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import DAGRunActions from '../DAGRunActions';
import {
  getDAGRunTerminateAction,
  isFailedAutoRetryPendingRun,
} from '../terminateAction';

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

describe('DAGRunActions', () => {
  it('treats running runs as stoppable', () => {
    expect(
      getDAGRunTerminateAction({
        status: Status.Running,
        autoRetryCount: 0,
        autoRetryLimit: 3,
      })
    ).toBe('stop');
  });

  it('treats failed runs with remaining auto retries as cancelable', () => {
    expect(
      isFailedAutoRetryPendingRun({
        status: Status.Failed,
        autoRetryCount: 1,
        autoRetryLimit: 3,
      })
    ).toBe(true);
    expect(
      getDAGRunTerminateAction({
        status: Status.Failed,
        autoRetryCount: 1,
        autoRetryLimit: 3,
      })
    ).toBe('cancel');
  });

  it('does not allow cancel once auto retries are exhausted', () => {
    expect(
      isFailedAutoRetryPendingRun({
        status: Status.Failed,
        autoRetryCount: 3,
        autoRetryLimit: 3,
      })
    ).toBe(false);
    expect(
      getDAGRunTerminateAction({
        status: Status.Failed,
        autoRetryCount: 3,
        autoRetryLimit: 3,
      })
    ).toBe('none');
  });

  it('does not allow cancel when no auto retry policy is configured', () => {
    expect(
      getDAGRunTerminateAction({
        status: Status.Failed,
        autoRetryCount: 0,
        autoRetryLimit: 0,
      })
    ).toBe('none');
  });

  it('shows cancel for failed runs with pending auto retries', () => {
    render(
      <DAGRunActions
        dagRun={{
          name: 'retry-dag',
          dagRunId: 'run-1',
          status: Status.Failed,
          autoRetryCount: 1,
          autoRetryLimit: 3,
        }}
        name="retry-dag"
        displayMode="full"
      />
    );

    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
  });
});
