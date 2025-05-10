import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import {
  DAGDetailsContent,
  DAGHeader,
} from '../../../features/dags/components/dag-details';
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

type WorkflowDetails = components['schemas']['WorkflowDetails'];

function DAGDetails() {
  const params = useParams<Params>();
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);
  const [searchParams] = useSearchParams();

  // Extract query parameters
  const workflowId = searchParams.get('workflowId');
  const stepName = searchParams.get('step');
  const childWorkflowId = searchParams.get('childWorkflowId');
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const fileName = params.fileName || '';

  // Determine active tab
  const tab = useMemo(() => params.tab || 'status', [params.tab]);

  // Format duration utility function
  const formatDuration = useCallback(
    (startDate: string, endDate: string): string => {
      if (!startDate || !endDate) return '--';

      const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
      const hours = Math.floor(duration.asHours());
      const minutes = duration.minutes();
      const seconds = duration.seconds();

      if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
      if (minutes > 0) return `${minutes}m ${seconds}s`;
      return `${seconds}s`;
    },
    []
  );

  // Navigate to status tab
  const navigateToStatusTab = useCallback(() => {
    if (fileName && tab !== 'status') {
      navigate(`/dags/${fileName}`);
    }
  }, [fileName, tab, navigate]);

  // Handle tab changes
  const handleTabChange = useCallback(
    (newTab: string) => {
      if (newTab === 'status' && fileName) {
        navigate(`/dags/${fileName}`);
      } else if (fileName) {
        navigate(`/dags/${fileName}/${newTab}`);
      }
    },
    [fileName, navigate]
  );

  // Fetch DAG details
  const { data: dagData, isLoading: isLoadingDag } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName },
      },
    },
    {
      refreshInterval: 2000,
    }
  );

  const dagName = dagData?.dag?.name || '';

  // Fetch specific workflow data if workflowId is provided
  const { data: workflowResponse, isLoading: isLoadingWorkflow } = useQuery(
    '/workflows/{name}/{workflowId}',
    {
      params: {
        path: {
          name: dagName,
          workflowId: workflowId || '',
        },
        query: { remoteNode },
      },
    },
    {
      isPaused: () => !dagName || !workflowId || !!childWorkflowId,
      refreshInterval: 2000,
      key: `/workflows/${dagName}/${workflowId}?remoteNode=${remoteNode}`,
    }
  );

  // Fetch child workflow data if needed
  const { data: childWorkflowResponse, isLoading: isLoadingChildWorkflow } =
    useQuery(
      '/workflows/{name}/{workflowId}/children/{childWorkflowId}',
      {
        params: {
          path: {
            name: dagName,
            workflowId: workflowId || '',
            childWorkflowId: childWorkflowId || '',
          },
          query: { remoteNode },
        },
      },
      {
        refreshInterval: 2000,
        isPaused: () => !childWorkflowId || !workflowId || !dagName,
        revalidateOnMount: true,
        revalidateIfStale: true,
      }
    );

  // Determine the current workflow to display
  const currentWorkflow = useMemo<WorkflowDetails | undefined>(() => {
    if (childWorkflowId && childWorkflowResponse?.workflowDetails) {
      return childWorkflowResponse.workflowDetails;
    }

    if (workflowId && !childWorkflowId && workflowResponse?.workflowDetails) {
      return workflowResponse.workflowDetails;
    }

    if (!childWorkflowId) {
      return dagData?.latestWorkflow;
    }

    return undefined;
  }, [
    childWorkflowId,
    childWorkflowResponse?.workflowDetails,
    workflowId,
    workflowResponse?.workflowDetails,
    dagData?.latestWorkflow,
  ]);

  // Root workflow context state
  const [rootWorkflowData, setRootWorkflowData] = useState<
    WorkflowDetails | undefined
  >(undefined);

  // Update root workflow data when current workflow changes
  useEffect(() => {
    if (currentWorkflow) {
      setRootWorkflowData(currentWorkflow);
    } else if (dagData?.latestWorkflow && !rootWorkflowData) {
      setRootWorkflowData(dagData.latestWorkflow);
    }
  }, [currentWorkflow, dagData?.latestWorkflow, rootWorkflowData]);

  // Determine if basic data is loading (no DAG data available at all)
  const isBasicLoading = useMemo(() => {
    return !fileName || isLoadingDag || !dagData || !dagData.dag;
  }, [fileName, isLoadingDag, dagData]);

  // Determine if content is loading (DAG data is available but workflow details are loading)
  const isContentLoading = useMemo(() => {
    // For non-status tabs, we don't need to wait for workflow data
    if (tab !== 'status') return false;

    // Child workflow loading state
    if (
      childWorkflowId &&
      (isLoadingChildWorkflow || !childWorkflowResponse?.workflowDetails)
    ) {
      return true;
    }

    // Specific workflow loading state (only for status tab)
    if (workflowId && !childWorkflowId) {
      if (isLoadingWorkflow || !workflowResponse?.workflowDetails) return true;
    }

    // No workflow data available
    if (!currentWorkflow && !dagData?.latestWorkflow) return true;

    return false;
  }, [
    tab,
    childWorkflowId,
    isLoadingChildWorkflow,
    childWorkflowResponse?.workflowDetails,
    workflowId,
    isLoadingWorkflow,
    workflowResponse?.workflowDetails,
    currentWorkflow,
    dagData?.latestWorkflow,
  ]);

  // Refresh function (placeholder for now)
  const refreshData = useCallback(() => {
    // This could be implemented to trigger a refresh of the data
    // For now it's a placeholder
  }, []);

  // If basic data is loading, show full page loading indicator
  if (isBasicLoading) {
    return <LoadingIndicator />;
  }

  // Determine which workflow to display in the header
  // We want to show the header even when content is loading
  const headerWorkflow = currentWorkflow || dagData?.latestWorkflow;

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshData,
        fileName,
        name: dagName,
      }}
    >
      <RootWorkflowContext.Provider
        value={{
          data: rootWorkflowData,
          setData: setRootWorkflowData,
        }}
      >
        <div className="w-full flex flex-col">
          {/* Always render the DAG Header when basic data is available */}
          {dagData?.dag && headerWorkflow && (
            <div className="mb-2">
              <DAGHeader
                dag={dagData.dag}
                currentWorkflow={headerWorkflow}
                fileName={fileName}
                refreshFn={refreshData}
                formatDuration={formatDuration}
                navigateToStatusTab={navigateToStatusTab}
              />
            </div>
          )}

          {/* Show loading indicator for content area only */}
          {isContentLoading ? (
            <div className="flex justify-center py-8">
              <LoadingIndicator />
            </div>
          ) : (
            dagData?.dag &&
            headerWorkflow && (
              <DAGDetailsContent
                fileName={fileName}
                dag={dagData.dag}
                currentWorkflow={headerWorkflow}
                refreshFn={refreshData}
                formatDuration={formatDuration}
                activeTab={tab}
                onTabChange={handleTabChange}
                workflowId={currentWorkflow?.workflowId}
                stepName={stepName}
                isModal={false}
                navigateToStatusTab={navigateToStatusTab}
                skipHeader={true} // Skip header since we're rendering it separately
              />
            )
          )}
        </div>
      </RootWorkflowContext.Provider>
    </DAGContext.Provider>
  );
}

export default DAGDetails;
