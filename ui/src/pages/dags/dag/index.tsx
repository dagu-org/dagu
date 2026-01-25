import React, { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { UnsavedChangesProvider } from '../../../contexts/UnsavedChangesContext';
import {
  DAGDetailsContent,
  DAGHeader,
} from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { RootDAGRunContext } from '../../../features/dags/contexts/RootDAGRunContext';
import { useQuery } from '../../../hooks/api';
import { useDAGSSE } from '../../../hooks/useDAGSSE';
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
  const subDAGRunId = searchParams.get('subDAGRunId');
  const queriedDAGRunName = searchParams.get('dagRunName');
  // Use remoteNode from URL if present, otherwise from app bar context
  const remoteNode = searchParams.get('remoteNode') || appBarContext.selectedRemoteNode || 'local';
  const fileName = params.fileName || '';

  // SSE for real-time updates with polling fallback
  const sseResult = useDAGSSE(fileName || '', !!fileName);
  const shouldPoll = sseResult.shouldUseFallback || !sseResult.isConnected;

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

  // Build URL with remote node parameter if needed
  const buildUrl = useCallback(
    (path: string) => {
      if (remoteNode && remoteNode !== 'local') {
        return `${path}?remoteNode=${encodeURIComponent(remoteNode)}`;
      }
      return path;
    },
    [remoteNode]
  );

  // Navigate to status tab
  const navigateToStatusTab = useCallback(() => {
    if (fileName && tab !== 'status') {
      navigate(buildUrl(`/dags/${fileName}`));
    }
  }, [fileName, tab, navigate, buildUrl]);

  // Handle tab changes
  const handleTabChange = useCallback(
    (newTab: string) => {
      if (newTab === 'status' && fileName) {
        navigate(buildUrl(`/dags/${fileName}`));
      } else if (fileName) {
        navigate(buildUrl(`/dags/${fileName}/${newTab}`));
      }
    },
    [fileName, navigate, buildUrl]
  );

  // Fetch DAG details - use polling only as fallback when SSE is not connected
  const { data: pollingDagData, mutate: mutateDag } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName },
      },
    },
    {
      refreshInterval: shouldPoll ? 2000 : 0,
      keepPreviousData: true,
      isPaused: () => !shouldPoll && sseResult.isConnected,
    }
  );

  // Use SSE data when available, otherwise fall back to polling
  const dagData = sseResult.data || pollingDagData;

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
        (!dagRunName && !queriedDAGRunName) || !dagRunId || !!subDAGRunId,
      refreshInterval: 2000,
    }
  );

  // Fetch sub DAG-run data if needed
  const { data: subDAGRunResponse, mutate: mutateSubDagRun } = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    {
      params: {
        path: {
          name: dagRunName,
          dagRunId: dagRunId || '',
          subDAGRunId: subDAGRunId || '',
        },
        query: { remoteNode },
      },
    },
    {
      refreshInterval: 2000,
      isPaused: () => !subDAGRunId || !dagRunId || !dagRunName,
    }
  );

  // Determine the current DAG-run to display
  let currentDAGRun: DAGRunDetails | undefined;
  if (subDAGRunId && subDAGRunResponse?.dagRunDetails) {
    currentDAGRun = subDAGRunResponse.dagRunDetails;
  } else if (dagRunId && !subDAGRunId && dagRunResponse?.dagRunDetails) {
    currentDAGRun = dagRunResponse.dagRunDetails;
  } else if (!subDAGRunId) {
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
    if (dagRunId && !subDAGRunId) {
      mutateDagRun();
    }
    if (subDAGRunId) {
      mutateSubDagRun();
    }
  }, [mutateDag, mutateDagRun, mutateSubDagRun, dagRunId, subDAGRunId]);

  // Determine which DAG-run to display in the header
  // We want to show the header even when content is loading
  const headerDAGRun = currentDAGRun || dagData?.latestDAGRun;

  return (
    <UnsavedChangesProvider>
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
                skipHeader={true}
                localDags={dagData?.localDags}
              />
            )}
          </div>
        </RootDAGRunContext.Provider>
      </DAGContext.Provider>
    </UnsavedChangesProvider>
  );
}

export default DAGDetails;
