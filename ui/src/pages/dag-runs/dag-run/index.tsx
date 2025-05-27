import React from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { DAGRunDetailsContent } from '../../../features/dag-runs/components/dag-run-details';
import { DAGRunContext } from '../../../features/dag-runs/contexts/DAGRunContext';
import { useQuery } from '../../../hooks/api';
import LoadingIndicator from '../../../ui/LoadingIndicator';

function DAGRunDetailsPage() {
  const { name, dagRunId = 'latest' } = useParams();
  const appBarContext = React.useContext(AppBarContext);
  const location = window.location.search;

  // Parse URL search params to check for childDAGRunId
  const searchParams = new URLSearchParams(location);
  const childDAGRunId = searchParams.get('childDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  // Determine the API endpoint based on whether this is a child DAG-run
  const endpoint = childDAGRunId
    ? '/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}'
    : '/dag-runs/{name}/{dagRunId}';

  // Fetch DAG-run details
  const { data, isLoading, mutate } = useQuery(
    endpoint,
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: childDAGRunId
          ? {
              name: parentName || '',
              dagRunId: parentDAGRunId || '',
              childDAGRunId: childDAGRunId,
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

  if (isLoading || !data) {
    return (
      <div className="flex items-center justify-center h-screen">
        <LoadingIndicator />
      </div>
    );
  }

  // Extract DAG-run details from the response
  // Both endpoints return data with a dagRunDetails property
  const dagRunDetails = data.dagRunDetails;

  // Use the actual DAG-run ID from the response for child DAG runs
  const displayDAGRunId = childDAGRunId || dagRunId || '';
  const displayName = childDAGRunId
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
