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

  // Fetch workflow details
  const { data, isLoading, mutate } = useQuery(
    '/workflows/{name}/{workflowId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
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

  return (
    <div className="container mx-auto p-4">
      <WorkflowContext.Provider
        value={{
          refresh: refreshFn,
          name: name || '',
          workflowId: workflowId || '',
        }}
      >
        <WorkflowDetailsContent
          name={name || ''}
          workflow={data.workflowDetails}
          refreshFn={refreshFn}
          workflowId={workflowId}
        />
      </WorkflowContext.Provider>
    </div>
  );
}

export default WorkflowDetailsPage;
