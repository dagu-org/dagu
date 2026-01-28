import React, { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Maximize2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { usePageContext } from '../../../../contexts/PageContext';
import { UnsavedChangesProvider } from '../../../../contexts/UnsavedChangesContext';
import { useQuery } from '../../../../hooks/api';
import { useDAGSSE } from '../../../../hooks/useDAGSSE';
import dayjs from '../../../../lib/dayjs';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { RootDAGRunContext } from '../../contexts/RootDAGRunContext';
import DAGDetailsContent from './DAGDetailsContent';

const POLLING_INTERVAL_MS = 2000;

function formatDuration(startDate: string, endDate: string): string {
  if (!startDate || !endDate) {
    return '--';
  }

  const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
  const hours = Math.floor(duration.asHours());
  const minutes = duration.minutes();
  const seconds = duration.seconds();

  if (hours > 0) {
    return `${hours}h ${minutes}m ${seconds}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

function getPollingInterval(notFound: boolean, shouldPoll: boolean): number {
  return notFound || !shouldPoll ? 0 : POLLING_INTERVAL_MS;
}

type Props = {
  fileName: string;
  onClose: () => void;
  onNavigate?: (direction: 'up' | 'down') => void;
};

type DAGRunDetails = components['schemas']['DAGRunDetails'];
type DAGDetailsData = ReturnType<typeof useDAGSSE>['data'];

function DAGDetailsPanel({ fileName, onClose, onNavigate }: Props): React.ReactElement | null {
  const navigate = useNavigate();
  const appBarContext = useContext(AppBarContext);
  const { setContext } = usePageContext();

  const [currentDAGRun, setCurrentDAGRun] = useState<DAGRunDetails | undefined>();
  const [activeTab, setActiveTab] = useState('status');
  const [notFound, setNotFound] = useState(false);
  const [lastValidData, setLastValidData] = useState<DAGDetailsData>(null);

  // Set page context for agent chat
  useEffect(() => {
    if (fileName) {
      setContext({
        dagFile: fileName,
        source: 'dag-details-panel',
      });
    }
    return () => {
      setContext(null);
    };
  }, [fileName, setContext]);

  // SSE for real-time updates with polling fallback
  const sseResult = useDAGSSE(fileName || '', !!fileName);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const shouldPoll = sseResult.shouldUseFallback || !sseResult.isConnected;

  const { data: pollingData, error, mutate } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName: fileName || '' },
      },
    },
    {
      refreshInterval: getPollingInterval(notFound, shouldPoll),
      keepPreviousData: true,
      isPaused: () => !shouldPoll && !notFound,
    }
  );

  const data = sseResult.data || pollingData;

  // Track data loading state and handle 404 errors
  useEffect(() => {
    if (error) {
      // Only set notFound for 404 errors when no cached data exists
      const is404 = (error as { status?: number })?.status === 404;
      if (is404 && !lastValidData) {
        setNotFound(true);
      }
    } else if (data) {
      setNotFound(false);
      setLastValidData(data as DAGDetailsData);
    }
  }, [error, data, lastValidData]);

  // Reset state when fileName changes
  useEffect(() => {
    setNotFound(false);
    setLastValidData(null); // Clear cached data when switching DAGs
    setActiveTab('status');
  }, [fileName]);

  const displayData = data || lastValidData;

  function refreshFn(): void {
    setTimeout(() => mutate(), 500);
  }

  function handleFullscreenClick(e?: React.MouseEvent): void {
    const tabPath = activeTab === 'status' ? '' : `/${activeTab}`;
    const url = `/dags/${fileName}${tabPath}`;

    if (e?.metaKey || e?.ctrlKey) {
      window.open(url, '_blank');
    } else {
      navigate(url);
    }
  }

  useEffect(() => {
    if (displayData) {
      setCurrentDAGRun(displayData.latestDAGRun);
    }
  }, [displayData]);

  // Keyboard shortcuts
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }

      if (event.key === 'Escape') {
        onClose();
        return;
      }

      if (event.key === 'f' || event.key === 'F') {
        handleFullscreenClick();
        return;
      }

      if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && onNavigate) {
        event.preventDefault();
        onNavigate(event.key === 'ArrowDown' ? 'down' : 'up');
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose, onNavigate, activeTab, fileName, navigate]);

  // Show error state if DAG not found
  if (notFound) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4 text-muted-foreground">
        <p className="text-sm">DAG not found or has been deleted.</p>
        <Button variant="outline" size="sm" onClick={onClose}>
          Close Tab
        </Button>
      </div>
    );
  }

  // Only show loading on initial load, not when switching DAGs
  // Gate on dag existence, not latestDAGRun, so DAGs with no runs can still be displayed
  if (!displayData?.dag) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingIndicator />
      </div>
    );
  }

  return (
    <UnsavedChangesProvider>
      <DAGContext.Provider
        value={{
          refresh: refreshFn,
          fileName: fileName || '',
          name: displayData.dag.name || '',
        }}
      >
        <RootDAGRunContext.Provider
          value={{
            data: currentDAGRun,
            setData: setCurrentDAGRun,
          }}
        >
          <div className="px-2 pt-2 w-full flex flex-col h-full overflow-hidden">
            <div className="flex justify-between items-center mb-2 flex-shrink-0">
              <p className="text-xs text-muted-foreground">
                Use{' '}
                <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
                  ↑
                </kbd>{' '}
                <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
                  ↓
                </kbd>{' '}
                to navigate DAGs
              </p>
              <div className="flex gap-2">
                <Button
                  size="icon"
                  onClick={handleFullscreenClick}
                  title="Open in fullscreen (F) - Cmd/Ctrl+Click to open in new tab"
                >
                  <Maximize2 className="h-4 w-4" />
                </Button>
                <Button size="icon" onClick={onClose} title="Close (Esc)">
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto overflow-x-hidden min-h-0">
              <DAGDetailsContent
                fileName={fileName}
                dag={displayData.dag}
                currentDAGRun={displayData.latestDAGRun}
                refreshFn={refreshFn}
                formatDuration={formatDuration}
                activeTab={activeTab}
                onTabChange={setActiveTab}
                dagRunId="latest"
                stepName={null}
                isModal={true}
                navigateToStatusTab={() => setActiveTab('status')}
                localDags={displayData.localDags}
              />
            </div>
          </div>
        </RootDAGRunContext.Provider>
      </DAGContext.Provider>
    </UnsavedChangesProvider>
  );
}

export default DAGDetailsPanel;
