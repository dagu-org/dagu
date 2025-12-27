import React from 'react';
import { Status } from '../../../api/v2/schema';
import { useQuery } from '../../../hooks/api';
import { AppBarContext } from '../../../contexts/AppBarContext';

export function useHasOutputs(
  dagName: string,
  dagRunId: string,
  status: Status
): boolean {
  const appBarContext = React.useContext(AppBarContext);

  const isCompleted =
    status === Status.Success ||
    status === Status.Failed ||
    status === Status.Aborted;

  const shouldFetch = isCompleted && !!dagName && !!dagRunId;

  const { data, error } = useQuery(
    '/dag-runs/{name}/{dagRunId}/outputs',
    {
      params: {
        query: { remoteNode: appBarContext.selectedRemoteNode || 'local' },
        path: { name: dagName || '', dagRunId: dagRunId || '' },
      },
    },
    {
      isPaused: () => !shouldFetch,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  );

  // Has outputs if completed, no error, and outputs object has keys
  const hasOutputs =
    shouldFetch &&
    !error &&
    !!data?.outputs &&
    Object.keys(data.outputs).length > 0;

  return hasOutputs;
}
