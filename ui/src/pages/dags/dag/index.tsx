import { useCallback, useContext, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { components } from '../../../api/v1/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { usePageContext } from '../../../contexts/PageContext';
import { UnsavedChangesProvider } from '../../../contexts/UnsavedChangesContext';
import { useWorkspace } from '../../../contexts/WorkspaceContext';
import {
  DAGDetailsContent,
  DAGHeader,
} from '../../../features/dags/components/dag-details';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { RootDAGRunContext } from '../../../features/dags/contexts/RootDAGRunContext';
import { useQuery } from '../../../hooks/api';
import { whenEnabled } from '../../../hooks/queryUtils';
import {
  liveFallbackOptions,
  useLiveConnection,
  useLiveDAG,
  useLiveDAGRuns,
} from '../../../hooks/useAppLive';
import dayjs from '../../../lib/dayjs';
import { matchesWorkspaceSelection } from '../../../lib/workspaceTags';
import LoadingIndicator from '../../../ui/LoadingIndicator';

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
  const { selectedWorkspace, workspaceReady } = useWorkspace();
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

  const liveState = useLiveConnection(!!fileName);

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

  const dagQueryEnabled = Boolean(fileName) && workspaceReady;
  // Fetch DAG details — SWR is the single source of truth, refreshed by live invalidations
  const { data: dagData, mutate: mutateDag } = useQuery(
    '/dags/{fileName}',
    whenEnabled(dagQueryEnabled, {
      params: {
        query: { remoteNode },
        path: { fileName },
      },
    }),
    liveFallbackOptions(liveState)
  );
  useLiveDAG(fileName, mutateDag, dagQueryEnabled);

  const isFilteredOut = Boolean(dagData?.dag) && !matchesWorkspaceSelection(
    dagData.dag.tags,
    selectedWorkspace
  );

  // Use dagRunName from URL if available, otherwise use the name from dagData
  const dagRunName = queriedDAGRunName || dagData?.dag?.name || '';
  const dagRunQueryEnabled = Boolean(
    dagRunName && dagRunId && !subDAGRunId && workspaceReady && !isFilteredOut
  );

  // Fetch specific DAG-run data if dagRunId is provided
  const { data: dagRunResponse, mutate: mutateDagRun } = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    whenEnabled(dagRunQueryEnabled, {
      params: {
        path: {
          name: dagRunName,
          dagRunId: dagRunId || '',
        },
        query: { remoteNode },
      },
    }),
    liveFallbackOptions(liveState)
  );
  useLiveDAGRuns(mutateDagRun, dagRunQueryEnabled);

  // Fetch sub DAG-run data if needed
  const subDAGRunQueryEnabled = Boolean(
    subDAGRunId && dagRunId && dagRunName && workspaceReady && !isFilteredOut
  );
  const { data: subDAGRunResponse, mutate: mutateSubDagRun } = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    whenEnabled(subDAGRunQueryEnabled, {
      params: {
        path: {
          name: dagRunName,
          dagRunId: dagRunId || '',
          subDAGRunId: subDAGRunId || '',
        },
        query: { remoteNode },
      },
    }),
    liveFallbackOptions(liveState)
  );
  useLiveDAGRuns(mutateSubDagRun, subDAGRunQueryEnabled);

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

  if (!workspaceReady) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingIndicator />
      </div>
    );
  }

  if (isFilteredOut) {
    return (
      <div className="max-w-7xl p-4">
        <div className="rounded-lg bg-muted p-6">
          <h2 className="text-lg font-semibold text-foreground mb-2">
            DAG Not Available
          </h2>
          <p className="text-muted-foreground">
            This DAG is outside the selected workspace.
          </p>
        </div>
      </div>
    );
  }

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
            {dagData?.dag && (
              <>
                <DAGHeader
                  dag={dagData.dag}
                  currentDAGRun={displayDAGRun}
                  fileName={fileName}
                  filePath={dagData.filePath}
                  refreshFn={refreshData}
                  formatDuration={formatDuration}
                  navigateToStatusTab={navigateToStatusTab}
                />
                <DAGDetailsContent
                  fileName={fileName}
                  filePath={dagData.filePath}
                  dag={dagData.dag}
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
