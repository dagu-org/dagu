import React, { useMemo } from 'react';
import { useLocation, useParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { DAGDetailsContent } from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { RunDetailsContext } from '../../../features/dags/contexts/DAGStatusContext';
import { useQuery } from '../../../hooks/api';
import dayjs from '../../../lib/dayjs';
import LoadingIndicator from '../../../ui/LoadingIndicator';

type Params = {
  fileId: string;
  name: string;
  tab?: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const appBarContext = React.useContext(AppBarContext);
  const { pathname } = useLocation();
  const { data, isLoading, mutate } = useQuery(
    '/dags/{fileId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: params.fileId || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );
  const [currentRun, setCurrentRun] = React.useState<
    components['schemas']['RunDetails'] | undefined
  >();
  const query = new URLSearchParams(window.location.search);
  const requestId =
    query.get('requestId') || data?.latestRun?.requestId || 'latest';
  const stepName = query.get('step');

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate, params.fileId]);

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.dag?.name || '');
      setCurrentRun(data.latestRun);
    }
  }, [data, appBarContext]);

  const tab = useMemo(() => {
    return params.tab || 'status';
  }, [params]);

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

  if (!params.fileId || isLoading || !data || !data.latestRun) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileId: params.fileId || '',
        name: data.dag?.name || '',
      }}
    >
      <RunDetailsContext.Provider
        value={{
          data: currentRun,
          setData: (status: components['schemas']['RunDetails']) => {
            setCurrentRun(status);
          },
        }}
      >
        <div className="w-full flex flex-col">
          {data.dag && (
            <DAGDetailsContent
              fileId={params.fileId || ''}
              dag={data.dag}
              latestRun={data.latestRun}
              refreshFn={refreshFn}
              formatDuration={formatDuration}
              activeTab={tab}
              requestId={requestId}
              stepName={stepName}
              isModal={false}
            />
          )}
        </div>
      </RunDetailsContext.Provider>
    </DAGContext.Provider>
  );
}
export default DAGDetails;
