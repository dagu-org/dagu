import React, { useCallback, useContext, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { usePageContext } from '../../../contexts/PageContext';
import { DAGRunDetailsContent } from '../../../features/dag-runs/components/dag-run-details';
import { DAGRunContext } from '../../../features/dag-runs/contexts/DAGRunContext';
import { useQuery } from '../../../hooks/api';

type ApiError = {
  response?: { status?: number };
  message?: string;
};

type ErrorDisplayProps = {
  error: unknown;
  name: string | undefined;
  dagRunId: string | undefined;
};

function ErrorDisplay({ error, name, dagRunId }: ErrorDisplayProps) {
  const apiError = error as ApiError;
  const statusCode = apiError?.response?.status;
  const isNotFound = statusCode === 404;

  const containerClass = isNotFound ? 'bg-muted' : 'bg-error-muted';
  const titleClass = isNotFound ? 'text-foreground' : 'text-error';
  const messageClass = isNotFound ? 'text-muted-foreground' : 'text-error';

  const title = isNotFound ? 'DAG Run Not Found' : 'Error Loading DAG Run';
  const message = isNotFound
    ? 'This DAG run may have been dequeued or removed. The previous state is no longer available.'
    : apiError?.message || 'Failed to load DAG run details';

  return (
    <div className="w-full px-4">
      <div className={`${containerClass} rounded-lg p-6 m-4`}>
        <h2 className={`text-lg font-semibold ${titleClass} mb-2`}>{title}</h2>
        <p className={messageClass}>{message}</p>
        {isNotFound && (
          <p className="text-sm text-muted-foreground mt-2">
            DAG: {name} | Run ID: {dagRunId}
          </p>
        )}
      </div>
    </div>
  );
}

function DAGRunDetailsPage() {
  const { name, dagRunId = 'latest' } = useParams();
  const appBarContext = useContext(AppBarContext);
  const { setContext } = usePageContext();

  const searchParams = new URLSearchParams(window.location.search);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  const canQuerySubDag = !!(subDAGRunId && parentDAGRunId && parentName);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    {
      params: {
        query: { remoteNode },
        path: {
          name: parentName as string,
          dagRunId: parentDAGRunId as string,
          subDAGRunId: subDAGRunId as string,
        },
      },
    },
    { refreshInterval: 2000, isPaused: () => !canQuerySubDag }
  );

  const dagRunQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    {
      params: {
        query: { remoteNode },
        path: {
          name: name || '',
          dagRunId: dagRunId || 'latest',
        },
      },
    },
    { refreshInterval: 2000, isPaused: () => canQuerySubDag }
  );

  const { data, error, mutate } = canQuerySubDag ? subDAGQuery : dagRunQuery;

  const refreshFn = useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const dagRunDetails = data?.dagRunDetails;
  const displayDAGRunId = subDAGRunId || dagRunId || '';

  function getDisplayName(): string {
    if (subDAGRunId) {
      return dagRunDetails?.name || parentName || '';
    }
    return name || '';
  }
  const displayName = getDisplayName();

  useEffect(() => {
    if (!displayName) {
      return;
    }
    setContext({
      dagFile: displayName,
      dagRunId: displayDAGRunId || undefined,
      dagRunName: displayName,
      source: 'dag-run-details-page',
    });
    return () => setContext(null);
  }, [displayName, displayDAGRunId, setContext]);

  if (error) {
    return <ErrorDisplay error={error} name={name} dagRunId={dagRunId} />;
  }

  if (!data || !dagRunDetails) {
    return null;
  }

  return (
    <div className="max-w-7xl px-4">
      <DAGRunContext.Provider
        value={{
          refresh: refreshFn,
          name: displayName,
          dagRunId: displayDAGRunId || '',
        }}
      >
        <DAGRunDetailsContent
          name={displayName}
          dagRun={dagRunDetails}
          refreshFn={refreshFn}
          dagRunId={displayDAGRunId}
        />
      </DAGRunContext.Provider>
    </div>
  );
}

export default DAGRunDetailsPage;
