import React, { useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { usePageContext } from '../../../contexts/PageContext';
import { DAGRunDetailsContent } from '../../../features/dag-runs/components/dag-run-details';
import { DAGRunContext } from '../../../features/dag-runs/contexts/DAGRunContext';
import { useQuery } from '../../../hooks/api';

function DAGRunDetailsPage() {
  const { name, dagRunId = 'latest' } = useParams();
  const appBarContext = React.useContext(AppBarContext);
  const { setContext } = usePageContext();
  const location = window.location.search;

  // Parse URL search params to check for subDAGRunId
  const searchParams = new URLSearchParams(location);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  // Guard: only query sub-DAG endpoint when all required params are present
  const canQuerySubDag = !!(subDAGRunId && parentDAGRunId && parentName);

  // Fetch sub-DAG-run details (only when all sub-DAG params are valid)
  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          name: parentName as string,
          dagRunId: parentDAGRunId as string,
          subDAGRunId: subDAGRunId as string,
        },
      },
    },
    { refreshInterval: 2000, isPaused: () => !canQuerySubDag }
  );

  // Fetch regular DAG-run details (only when not querying sub-DAG)
  const dagRunQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          name: name || '',
          dagRunId: dagRunId || 'latest',
        },
      },
    },
    { refreshInterval: 2000, isPaused: () => canQuerySubDag }
  );

  // Use the appropriate query based on whether this is a sub-DAG-run
  const { data, error, mutate } = canQuerySubDag ? subDAGQuery : dagRunQuery;

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  // Handle 404 error for dequeued DAG runs
  if (error) {
    const statusCode = (error as { response?: { status?: number } })?.response?.status;
    if (statusCode === 404) {
      return (
        <div className="w-full px-4">
          <div className="bg-muted rounded-lg p-6 m-4">
            <h2 className="text-lg font-semibold text-foreground mb-2">
              DAG Run Not Found
            </h2>
            <p className="text-muted-foreground">
              This DAG run may have been dequeued or removed. The previous state is no longer available.
            </p>
            <p className="text-sm text-muted-foreground mt-2">
              DAG: {name} | Run ID: {dagRunId}
            </p>
          </div>
        </div>
      );
    }
    // For other errors, show a generic error message
    return (
      <div className="w-full px-4">
        <div className="bg-error-muted rounded-lg p-6 m-4">
          <h2 className="text-lg font-semibold text-error mb-2">
            Error Loading DAG Run
          </h2>
          <p className="text-error">
            {(error as { message?: string })?.message || 'Failed to load DAG run details'}
          </p>
        </div>
      </div>
    );
  }

  if (!data) {
    return null;
  }

  // Extract DAG-run details from the response
  // Both endpoints return data with a dagRunDetails property
  const dagRunDetails = data.dagRunDetails;

  // Use the actual DAG-run ID from the response for sub DAG runs
  const displayDAGRunId = subDAGRunId || dagRunId || '';
  const displayName = subDAGRunId
    ? dagRunDetails?.name || parentName || ''
    : name || '';

  // Set page context for agent chat
  useEffect(() => {
    if (displayName) {
      setContext({
        dagFile: displayName,
        dagRunId: displayDAGRunId || undefined,
        dagRunName: displayName,
        source: 'dag-run-details-page',
      });
    }
    return () => {
      setContext(null);
    };
  }, [displayName, displayDAGRunId, setContext]);

  return (
    <div className="w-full px-4">
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
