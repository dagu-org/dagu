import React from 'react';
import { AppBarContext } from '../../contexts/AppBarContext';
import WorkflowTable from '../../features/workflows/components/workflow-list/WorkflowTable';
import { useQuery } from '../../hooks/api';
import LoadingIndicator from '../../ui/LoadingIndicator';
import Title from '../../ui/Title';

function Workflows() {
  const appBarContext = React.useContext(AppBarContext);

  React.useEffect(() => {
    appBarContext.setTitle('Workflows');
  }, [appBarContext]);

  const { data, isLoading } = useQuery('/workflows', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
    },
  });

  return (
    <div className="flex flex-col">
      <Title>Workflows</Title>
      {isLoading ? (
        <LoadingIndicator />
      ) : (
        <WorkflowTable workflows={data?.workflows || []} />
      )}
    </div>
  );
}

export default Workflows;
