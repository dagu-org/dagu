import { Status } from '@/api/v1/schema';

type AutoRetryCancelableRun = {
  status?: Status | null;
  autoRetryCount?: number | null;
  autoRetryLimit?: number | null;
};

export type DAGRunTerminateAction = 'none' | 'stop' | 'cancel';

export function isFailedAutoRetryPendingRun(
  run?: AutoRetryCancelableRun | null
): boolean {
  const limit = run?.autoRetryLimit ?? 0;
  const count = Math.max(run?.autoRetryCount ?? 0, 0);

  return run?.status === Status.Failed && limit > 0 && count < limit;
}

export function getDAGRunTerminateAction(
  run?: AutoRetryCancelableRun | null
): DAGRunTerminateAction {
  if (run?.status === Status.Running) {
    return 'stop';
  }
  if (isFailedAutoRetryPendingRun(run)) {
    return 'cancel';
  }
  return 'none';
}
