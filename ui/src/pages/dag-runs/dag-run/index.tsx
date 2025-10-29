import React from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { DAGRunDetailsContent } from '../../../features/dag-runs/components/dag-run-details';
import { DAGRunContext } from '../../../features/dag-runs/contexts/DAGRunContext';
import { useQuery } from '../../../hooks/api';

function DAGRunDetailsPage() {
  const { name, dagRunId = 'latest' } = useParams();
  const appBarContext = React.useContext(AppBarContext);
  const location = window.location.search;

  // Parse URL search params to check for subDAGRunId
  const searchParams = new URLSearchParams(location);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  // Determine the API endpoint based on whether this is a sub DAG-run
  const endpoint = subDAGRunId
    ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}'
    : '/dag-runs/{name}/{dagRunId}';

  // Fetch DAG-run details
  const { data, error, mutate } = useQuery(
    endpoint,
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: subDAGRunId
          ? {
              name: parentName || '',
              dagRunId: parentDAGRunId || '',
              subDAGRunId: subDAGRunId,
            }
          : {
              name: name || '',
              dagRunId: dagRunId || 'latest',
            },
      },
    },
    { refreshInterval: 2000 }
  );

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  // Handle 404 error for dequeued DAG runs
  if (error) {
    const statusCode = (error as { response?: { status?: number } })?.response?.status;
    if (statusCode === 404) {
      return (
        <div className="container mx-auto">
          <div className="bg-slate-100 dark:bg-slate-800 rounded-lg p-6 m-4">
            <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-200 mb-2">
              DAG Run Not Found
            </h2>
            <p className="text-slate-600 dark:text-slate-400">
              This DAG run may have been dequeued or removed. The previous state is no longer available.
            </p>
            <p className="text-sm text-slate-500 dark:text-slate-500 mt-2">
              DAG: {name} | Run ID: {dagRunId}
            </p>
          </div>
        </div>
      );
    }
    // For other errors, show a generic error message
    return (
      <div className="container mx-auto">
        <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-6 m-4">
          <h2 className="text-lg font-semibold text-red-800 dark:text-red-200 mb-2">
            Error Loading DAG Run
          </h2>
          <p className="text-red-600 dark:text-red-400">
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

  return (
    <div className="container mx-auto">
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
