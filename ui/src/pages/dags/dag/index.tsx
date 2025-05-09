import React, { useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { DAGDetailsContent } from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { WorkflowDetailsContext } from '../../../features/dags/contexts/DAGStatusContext';
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
  const { data, isLoading } = useQuery(
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
    { refreshInterval: 2000 }
  );
  const [currentWorkflow, setCurrentWorkflow] = React.useState<
    components['schemas']['WorkflowDetails'] | undefined
  >();
  const query = new URLSearchParams(window.location.search);
  const workflowId =
    query.get('workflowId') || data?.latestWorkflow?.workflowId || 'latest';
  const stepName = query.get('step');

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.dag?.name || '');
      setCurrentWorkflow(data.latestWorkflow);
    }
  }, [data, appBarContext]);

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

  if (!params.fileName || isLoading || !data || !data.latestWorkflow) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: () => {},
        fileName: params.fileName || '',
        name: data.dag?.name || '',
      }}
    >
      <WorkflowDetailsContext.Provider
        value={{
          data: currentWorkflow,
          setData: (status: components['schemas']['WorkflowDetails']) => {
            setCurrentWorkflow(status);
          },
        }}
      >
        <div className="w-full flex flex-col">
          {data.dag && (
            <DAGDetailsContent
              fileName={params.fileName || ''}
              dag={data.dag}
              latestWorkflow={data.latestWorkflow}
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
      </WorkflowDetailsContext.Provider>
    </DAGContext.Provider>
  );
}
export default DAGDetails;
