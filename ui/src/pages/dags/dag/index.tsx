import React, { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import {
  DAGDetailsContent,
  DAGHeader,
} from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { RootDAGRunContext } from '../../../features/dags/contexts/RootDAGRunContext';
import { useQuery } from '../../../hooks/api';
import dayjs from '../../../lib/dayjs';

type Params = {
  fileName: string;
  name: string;
  tab?: string;
};

type DAGRunDetails = components['schemas']['DAGRunDetails'];

function DAGDetails() {
  const params = useParams<Params>();
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);
  const [searchParams] = useSearchParams();

  // Extract query parameters
  const dagRunId = searchParams.get('dagRunId');
  const stepName = searchParams.get('step');
  const childDAGRunId = searchParams.get('childDAGRunId');
  const queriedDAGRunName = searchParams.get('dagRunName');
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const fileName = params.fileName || '';

  // Determine active tab
  const tab = params.tab || 'status';

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
  const { data: dagData, mutate: mutateDag } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName },
      },
    },
    {
      refreshInterval: 1000,
    }
  );

  // Use dagRunName from URL if available, otherwise use the name from dagData
  const dagRunName = queriedDAGRunName || dagData?.dag?.name || '';

  // Fetch specific DAG-run data if dagRunId is provided
  const { data: dagRunResponse, mutate: mutateDagRun } = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    {
      params: {
        path: {
          name: dagRunName,
          dagRunId: dagRunId || '',
        },
        query: { remoteNode },
      },
    },
    {
      isPaused: () =>
        (!dagRunName && !queriedDAGRunName) || !dagRunId || !!childDAGRunId,
      refreshInterval: 1000,
    }
  );

  // Fetch child DAG-run data if needed
  const { data: childDAGRunResponse, mutate: mutateChildDagRun } = useQuery(
    '/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}',
    {
      params: {
        path: {
          name: dagRunName,
          dagRunId: dagRunId || '',
          childDAGRunId: childDAGRunId || '',
        },
        query: { remoteNode },
      },
    },
    {
      refreshInterval: 1000,
      isPaused: () => !childDAGRunId || !dagRunId || !dagRunName,
    }
  );

  // Determine the current DAG-run to display
  let currentDAGRun: DAGRunDetails | undefined;
  if (childDAGRunId && childDAGRunResponse?.dagRunDetails) {
    currentDAGRun = childDAGRunResponse.dagRunDetails;
  } else if (dagRunId && !childDAGRunId && dagRunResponse?.dagRunDetails) {
    currentDAGRun = dagRunResponse.dagRunDetails;
  } else if (!childDAGRunId) {
    currentDAGRun = dagData?.latestDAGRun;
  }

  // Root DAG-run context state
  const [rootDAGRunData, setRootDAGRunData] = useState<
    DAGRunDetails | undefined
  >(undefined);

  // Update root DAG-run data when current DAG-run changes
  // This is now the only place that updates the rootDAGRunContext
  // The history page only changes the URL parameters
  useEffect(() => {
    // Set the initial value if rootDAGRunData is undefined
    if (!rootDAGRunData) {
      if (currentDAGRun) {
        setRootDAGRunData(currentDAGRun);
      } else if (dagData?.latestDAGRun) {
        setRootDAGRunData(dagData.latestDAGRun);
      }
    }
    // Always update when currentDAGRun changes, regardless of the tab
    // This ensures the header is updated when navigating through history
    else if (currentDAGRun) {
      setRootDAGRunData(currentDAGRun);
    } else if (dagData?.latestDAGRun) {
      setRootDAGRunData(dagData.latestDAGRun);
    }
  }, [currentDAGRun, dagData?.latestDAGRun, rootDAGRunData]);

  // Refresh function
  const refreshData = useCallback(() => {
    mutateDag();
    if (dagRunId && !childDAGRunId) {
      mutateDagRun();
    }
    if (childDAGRunId) {
      mutateChildDagRun();
    }
  }, [mutateDag, mutateDagRun, mutateChildDagRun, dagRunId, childDAGRunId]);

  // Determine which DAG-run to display in the header
  // We want to show the header even when content is loading
  const headerDAGRun = currentDAGRun || dagData?.latestDAGRun;

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshData,
        fileName,
        name: dagRunName,
      }}
    >
      <RootDAGRunContext.Provider
        value={{
          data: rootDAGRunData,
          setData: setRootDAGRunData,
        }}
      >
        <div className="w-full flex flex-col">
          {/* Always render the DAG Header when basic data is available */}
          {dagData?.dag && headerDAGRun && (
            <DAGHeader
              dag={dagData.dag}
              currentDAGRun={headerDAGRun}
              fileName={fileName}
              refreshFn={refreshData}
              formatDuration={formatDuration}
              navigateToStatusTab={navigateToStatusTab}
            />
          )}

          {/* Render content */}
          {dagData?.dag && headerDAGRun && (
            <DAGDetailsContent
              fileName={fileName}
              dag={dagData.dag}
              currentDAGRun={headerDAGRun}
              refreshFn={refreshData}
              formatDuration={formatDuration}
              activeTab={tab}
              onTabChange={handleTabChange}
              dagRunId={currentDAGRun?.dagRunId}
              stepName={stepName}
              isModal={false}
              navigateToStatusTab={navigateToStatusTab}
              skipHeader={true} // Skip header since we're rendering it separately
            />
          )}
        </div>
      </RootDAGRunContext.Provider>
    </DAGContext.Provider>
  );
}

export default DAGDetails;
