import React, { useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
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

  // Extract query parameters
  const query = new URLSearchParams(window.location.search);
  const workflowId = query.get('workflowId') || 'latest';
  const stepName = query.get('step');

  // Extract child workflow parameters
  const childWorkflowId = query.get('childWorkflowId');
  const rootWorkflowName = query.get('rootWorkflowName');
  const rootWorkflowId = query.get('rootWorkflowId');

  // Fetch DAG details
  const { data: dagData, isLoading: isLoadingDagData } = useQuery(
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
      refreshInterval: childWorkflowId ? 0 : 2000, // Don't auto-refresh for child workflows
    }
  );

  // Fetch specific workflow data if workflowId is provided and not 'latest'
  const { data: workflowResponse, isLoading: isLoadingWorkflow } = useQuery(
    '/workflows/{name}/{workflowId}',
    {
      params: {
        path: {
          name: dagData?.dag?.name || '',
          workflowId: workflowId,
        },
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      enabled: !!(
        dagData?.dag?.name &&
        dagData.dag.name.trim() !== '' &&
        workflowId !== 'latest'
      ),
      refreshInterval: 2000,
    }
  );

  // Fetch child workflow data if needed
  const { data: childWorkflowResponse, isLoading: isLoadingChildWorkflow } =
    useQuery(
      '/workflows/{name}/{workflowId}/children/{childWorkflowId}',
      {
        params: {
          path: {
            name: rootWorkflowName || '',
            workflowId: rootWorkflowId || '',
            childWorkflowId: childWorkflowId || '',
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
      },
      {
        // Only fetch if all required parameters are present and not empty strings
        enabled: !!(
          childWorkflowId &&
          rootWorkflowName &&
          rootWorkflowName.trim() !== '' &&
          rootWorkflowId &&
          rootWorkflowId.trim() !== ''
        ),
        // Don't auto-refresh for child workflows
        refreshInterval: 0,
      }
    );

  // Process child workflow data
  const childWorkflow = useMemo(() => {
    if (!childWorkflowResponse) return undefined;

    try {
      // Try to handle different possible response structures
      if ('workflowDetails' in childWorkflowResponse) {
        // If the response has a workflowDetails property
        return childWorkflowResponse.workflowDetails as unknown as components['schemas']['WorkflowDetails'];
      } else if ('nodes' in childWorkflowResponse) {
        // If the response already has the expected structure
        return childWorkflowResponse as unknown as components['schemas']['WorkflowDetails'];
      }
    } catch (err) {
      console.error('Error processing child workflow data:', err);
    }

    return undefined;
  }, [childWorkflowResponse]);

  // Update the title based on workflow data
  React.useEffect(() => {
    if (dagData?.dag) {
      if (childWorkflowId && childWorkflow && dagData.latestWorkflow) {
        // Find the parent step that has this child workflow
        const parentStep = dagData.latestWorkflow.nodes?.find((node) =>
          node.children?.some((child) => child.workflowId === childWorkflowId)
        );
        const childDagName = parentStep?.step.run || 'Child Workflow';
        appBarContext.setTitle(`${dagData.dag.name || ''} → ${childDagName}`);
      } else {
        // Regular workflow (not a child)
        appBarContext.setTitle(dagData.dag.name || '');
      }
    }
  }, [dagData, childWorkflow, childWorkflowId, appBarContext]);

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

  // Process workflow data
  const workflowData = useMemo(() => {
    if (!workflowResponse) return undefined;

    try {
      if ('workflow' in workflowResponse) {
        return workflowResponse;
      }
    } catch (err) {
      console.error('Error processing workflow data:', err);
    }

    return undefined;
  }, [workflowResponse]);

  if (
    !params.fileName ||
    isLoadingDagData ||
    (workflowId !== 'latest' && isLoadingWorkflow) ||
    (childWorkflowId && isLoadingChildWorkflow) ||
    !dagData ||
    !dagData.latestWorkflow
  ) {
    return <LoadingIndicator />;
  }

  // Determine the current workflow to display
  const currentWorkflow =
    childWorkflow ||
    (workflowData?.workflow as components['schemas']['WorkflowDetails']) ||
    dagData.latestWorkflow;

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
          data: dagData.latestWorkflow, // Use the latest workflow as the root by default
          setData: () => {}, // No-op since we're not using state
        }}
      >
        <div className="w-full flex flex-col">
          {dagData.dag && (
            <DAGDetailsContent
              fileName={params.fileName || ''}
              dag={dagData.dag}
              currentWorkflow={currentWorkflow}
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
              workflowId={workflowId}
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
