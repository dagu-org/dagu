import { useCallback, useContext, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v1/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { usePageContext } from '../../../contexts/PageContext';
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
  const appBarContext = useContext(AppBarContext);
  const { setContext } = usePageContext();
  const [searchParams] = useSearchParams();

  const dagRunId = searchParams.get('dagRunId');
  const stepName = searchParams.get('step');
  const subDAGRunId = searchParams.get('subDAGRunId');
  const queriedDAGRunName = searchParams.get('dagRunName');
  const remoteNode = searchParams.get('remoteNode') || appBarContext.selectedRemoteNode || 'local';
  const fileName = params.fileName || '';

  // Set page context for agent chat
  useEffect(() => {
    if (fileName) {
      setContext({
        dagFile: fileName,
        dagRunId: dagRunId || undefined,
        source: 'dag-details-page',
      });
    }
    return () => {
      setContext(null);
    };
  }, [fileName, dagRunId, setContext]);

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

  // Handle tab changes - navigates to the appropriate URL for the given tab
  const handleTabChange = useCallback(
    (newTab: string) => {
      if (!fileName) return;
      const path = newTab === 'status' ? `/dags/${fileName}` : `/dags/${fileName}/${newTab}`;
      navigate(buildUrl(path));
    },
    [fileName, navigate, buildUrl]
  );

  // Navigate to status tab - convenience wrapper for handleTabChange
  const navigateToStatusTab = useCallback(() => {
    if (tab !== 'status') {
      handleTabChange('status');
    }
  }, [tab, handleTabChange]);

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

  // Determine the current DAG-run to display based on URL parameters
  function getCurrentDAGRun(): DAGRunDetails | undefined {
    if (subDAGRunId) {
      return subDAGRunResponse?.dagRunDetails;
    }
    if (dagRunId) {
      return dagRunResponse?.dagRunDetails;
    }
    return dagData?.latestDAGRun;
  }
  const currentDAGRun = getCurrentDAGRun();

  // Root DAG-run context state for header display
  const [rootDAGRunData, setRootDAGRunData] = useState<DAGRunDetails | undefined>(undefined);

  // Update root DAG-run data when current DAG-run or latest DAG-run changes
  useEffect(() => {
    const newData = currentDAGRun || dagData?.latestDAGRun;
    if (newData) {
      setRootDAGRunData(newData);
    }
  }, [currentDAGRun, dagData?.latestDAGRun]);

  // Refresh all relevant data based on current view
  const refreshData = useCallback(() => {
    mutateDag();
    if (subDAGRunId) {
      mutateSubDagRun();
    } else if (dagRunId) {
      mutateDagRun();
    }
  }, [mutateDag, mutateDagRun, mutateSubDagRun, dagRunId, subDAGRunId]);

  // Determine which DAG-run to display - fallback to latest when specific run is loading
  const displayDAGRun = currentDAGRun || dagData?.latestDAGRun;
  const isDataReady = dagData?.dag && displayDAGRun;

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
          <div className="max-w-7xl flex flex-col">
            {isDataReady && (
              <>
                <DAGHeader
                  dag={dagData.dag!}
                  currentDAGRun={displayDAGRun}
                  fileName={fileName}
                  refreshFn={refreshData}
                  formatDuration={formatDuration}
                  navigateToStatusTab={navigateToStatusTab}
                />
                <DAGDetailsContent
                  fileName={fileName}
                  dag={dagData.dag!}
                  currentDAGRun={displayDAGRun}
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
              </>
            )}
          </div>
        </RootDAGRunContext.Provider>
      </DAGContext.Provider>
    </UnsavedChangesProvider>
  );
}

export default DAGDetails;
