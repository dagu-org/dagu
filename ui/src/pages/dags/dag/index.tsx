import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { DAGDetailsContent } from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { RootWorkflowContext } from '../../../features/dags/contexts/RootWorkflowContext';
import { useQuery } from '../../../hooks/api';
import dayjs from '../../../lib/dayjs';
import LoadingIndicator from '../../../ui/LoadingIndicator';

type Params = {
  fileName: string;
  name: string;
  tab?: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);

  // Use React Router's useSearchParams hook to get query parameters
  const [searchParams] = useSearchParams();
  const workflowId = searchParams.get('workflowId');
  const stepName = searchParams.get('step');
  const childWorkflowId = searchParams.get('childWorkflowId');

  // Fetch DAG details
  const { data: dagData } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileName: params.fileName || '',
        },
      },
    },
    {
      refreshInterval: 2000,
    }
  );

  // Fetch specific workflow data if workflowId is provided and not 'latest'
  const { data: workflowResponse, isLoading: isLoadingWorkflow } = useQuery(
    '/workflows/{name}/{workflowId}',
    {
      params: {
        path: {
          name: dagData?.dag?.name || '',
          workflowId: workflowId || '',
        },
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      isPaused: () =>
        !(
          dagData?.dag?.name &&
          dagData.dag.name.trim() !== '' &&
          workflowId &&
          !childWorkflowId
        ),
      refreshInterval: 2000,
      key: `/workflows/${dagData?.dag?.name}/${workflowId}?remoteNode=${appBarContext.selectedRemoteNode || 'local'}`,
    }
  );

  // Fetch child workflow data if needed
  const { data: childWorkflowResponse, isLoading: isLoadingChildWorkflow } =
    useQuery(
      '/workflows/{name}/{workflowId}/children/{childWorkflowId}',
      {
        params: {
          path: {
            name: dagData?.dag?.name || '',
            workflowId: workflowId || '',
            childWorkflowId: childWorkflowId || '',
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
      },
      {
        refreshInterval: 2000,
        isPaused: () => !(childWorkflowId && workflowId && dagData?.dag?.name),
        revalidateOnMount: true,
        revalidateIfStale: true,
      }
    );

  const tab = useMemo(() => {
    return params.tab || 'status';
  }, [params]);

  // Function to navigate to the status tab
  const navigateToStatusTab = () => {
    if (params.fileName && tab !== 'status') {
      navigate(`/dags/${params.fileName}`);
    }
  };

  const formatDuration = (startDate: string, endDate: string) => {
    if (!startDate || !endDate) return '--';
    const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
  };

  // Determine the current workflow to display
  let currentWorkflow: components['schemas']['WorkflowDetails'] | undefined;

  if (childWorkflowId && childWorkflowResponse?.workflowDetails) {
    currentWorkflow = childWorkflowResponse.workflowDetails;
  } else if (
    workflowId &&
    !childWorkflowId &&
    workflowResponse?.workflowDetails
  ) {
    currentWorkflow = workflowResponse.workflowDetails;
  } else if (!childWorkflowId) {
    // Only use latest workflow if not trying to view a child workflow
    currentWorkflow = dagData?.latestWorkflow;
  }

  // Create a state for the root workflow context
  const [rootWorkflowData, setRootWorkflowData] = useState<
    components['schemas']['WorkflowDetails'] | undefined
  >(dagData?.latestWorkflow);

  // Update the root workflow context when currentWorkflow changes
  useEffect(() => {
    if (currentWorkflow) {
      setRootWorkflowData(currentWorkflow);
    }
  }, [currentWorkflow]);

  // Show loading indicator while data is being fetched
  if (!params.fileName || !dagData) {
    return <LoadingIndicator />;
  }

  // Show loading indicator for child workflow if needed
  if (
    childWorkflowId &&
    (!childWorkflowResponse?.workflowDetails || isLoadingChildWorkflow)
  ) {
    return <LoadingIndicator />;
  }

  // Show loading indicator for specific workflow if needed
  if (workflowId && !childWorkflowId && isLoadingWorkflow && tab === 'status') {
    return <LoadingIndicator />;
  }

  if (
    workflowId &&
    !childWorkflowId &&
    !workflowResponse?.workflowDetails &&
    tab === 'status'
  ) {
    return <LoadingIndicator />;
  }

  // Show loading indicator if no workflow data is available for status tab
  if (!currentWorkflow && tab === 'status') {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: () => {},
        fileName: params.fileName || '',
        name: dagData.dag?.name || '',
      }}
    >
      <RootWorkflowContext.Provider
        value={{
          data: rootWorkflowData,
          setData: setRootWorkflowData,
        }}
      >
        <div className="w-full flex flex-col">
          {dagData.dag && (
            <DAGDetailsContent
              fileName={params.fileName || ''}
              dag={dagData.dag}
              currentWorkflow={currentWorkflow || dagData.latestWorkflow}
              refreshFn={() => {}}
              formatDuration={formatDuration}
              activeTab={tab}
              onTabChange={(newTab) => {
                if (newTab === 'status' && params.fileName) {
                  navigate(`/dags/${params.fileName}`);
                } else if (params.fileName) {
                  navigate(`/dags/${params.fileName}/${newTab}`);
                }
              }}
              workflowId={currentWorkflow?.workflowId}
              stepName={stepName}
              isModal={false}
              navigateToStatusTab={navigateToStatusTab}
            />
          )}
        </div>
      </RootWorkflowContext.Provider>
    </DAGContext.Provider>
  );
}

export default DAGDetails;
