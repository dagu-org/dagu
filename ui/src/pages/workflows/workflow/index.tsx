import React from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { WorkflowDetailsContent } from '../../../features/workflows/components/workflow-details';
import { WorkflowContext } from '../../../features/workflows/contexts/WorkflowContext';
import { useQuery } from '../../../hooks/api';
import LoadingIndicator from '../../../ui/LoadingIndicator';

function WorkflowDetailsPage() {
  const { name, workflowId = 'latest' } = useParams();
  const appBarContext = React.useContext(AppBarContext);
  const location = window.location.search;

  // Parse URL search params to check for childWorkflowId
  const searchParams = new URLSearchParams(location);
  const childWorkflowId = searchParams.get('childWorkflowId');
  const parentWorkflowId = searchParams.get('workflowId');
  const parentName = searchParams.get('workflowName') || name;

  // Determine the API endpoint based on whether this is a child workflow
  const endpoint = childWorkflowId
    ? '/workflows/{name}/{workflowId}/children/{childWorkflowId}'
    : '/workflows/{name}/{workflowId}';

  // Fetch workflow details
  const { data, isLoading, mutate } = useQuery(
    endpoint,
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: childWorkflowId
          ? {
              name: parentName || '',
              workflowId: parentWorkflowId || '',
              childWorkflowId: childWorkflowId,
            }
          : {
              name: name || '',
              workflowId: workflowId || 'latest',
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

  // Extract workflow details from the response
  // Both endpoints return data with a workflowDetails property
  const workflowDetails = data.workflowDetails;

  // Use the actual workflow ID from the response for child workflows
  const displayWorkflowId = childWorkflowId || workflowId || '';
  const displayName = childWorkflowId
    ? workflowDetails?.name || parentName || ''
    : name || '';

  return (
    <div className="container mx-auto p-4">
      <WorkflowContext.Provider
        value={{
          refresh: refreshFn,
          name: displayName,
          workflowId: displayWorkflowId || '',
        }}
      >
        <WorkflowDetailsContent
          name={displayName}
          workflow={workflowDetails}
          refreshFn={refreshFn}
          workflowId={displayWorkflowId}
        />
      </WorkflowContext.Provider>
    </div>
  );
}

export default WorkflowDetailsPage;
