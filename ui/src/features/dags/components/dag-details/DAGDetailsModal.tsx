import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v1/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import { useDAGSSE } from '../../../../hooks/useDAGSSE';
import dayjs from '../../../../lib/dayjs';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { RootDAGRunContext } from '../../contexts/RootDAGRunContext';
import DAGDetailsContent from './DAGDetailsContent';

type Props = {
  fileName: string;
  isOpen: boolean;
  onClose: () => void;
};

function DAGDetailsModal({ fileName, isOpen, onClose }: Props): React.ReactElement | null {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);
  const [currentDAGRun, setCurrentDAGRun] = React.useState<
    components['schemas']['DAGRunDetails'] | undefined
  >();
  const [activeTab, setActiveTab] = React.useState('status');
  const [dagRunId] = React.useState<string>('latest');
  const [stepName] = React.useState<string | null>(null);

  // Function to navigate to status tab
  const navigateToStatusTab = React.useCallback(() => {
    setActiveTab('status');
  }, []);

  // SSE for real-time updates (only when modal is open)
  const sseResult = useDAGSSE(fileName || '', isOpen && !!fileName);

  // Polling fallback (only when SSE fails or not connected)
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const usePolling = sseResult.shouldUseFallback || !sseResult.isConnected;

  const { data: pollingData, mutate } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName: fileName || '' },
      },
    },
    {
      refreshInterval: usePolling ? 2000 : 0,
      keepPreviousData: true,
      isPaused: () => !isOpen || (!usePolling && sseResult.isConnected),
    }
  );

  // Use SSE data when available, otherwise polling
  const data = sseResult.data || pollingData;

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = React.useCallback(
    (e?: React.MouseEvent) => {
      const url =
        activeTab === 'status'
          ? `/dags/${fileName}`
          : `/dags/${fileName}/${activeTab}`;

      if (e?.metaKey || e?.ctrlKey) {
        window.open(url, '_blank');
      } else {
        navigate(url);
      }
    },
    [activeTab, fileName, navigate]
  );

  React.useEffect(() => {
    if (data) {
      setCurrentDAGRun(data.latestDAGRun);
    }
  }, [data]);

  // Keyboard shortcuts
  React.useEffect(() => {
    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }

      // Don't capture browser shortcuts like Ctrl/Cmd+F
      if (event.metaKey || event.ctrlKey) {
        return;
      }

      switch (event.key) {
        case 'Escape':
          onClose();
          break;
        case 'f':
        case 'F':
          handleFullscreenClick();
          break;
      }
    }

    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose, handleFullscreenClick]);

  const formatDuration = (startDate: string, endDate: string): string => {
    if (!startDate || !endDate) return '--';

    const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
  };

  if (!isOpen) return null;

  // Show loading when no data is available
  // Gate on dag existence, not latestDAGRun, so DAGs with no runs can still be displayed
  if (!data?.dag) {
    return (
      <div className="fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-background border-l border-border z-50 flex items-center justify-center">
        <LoadingIndicator />
      </div>
    );
  }

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 h-screen w-screen bg-black/20 z-40"
        onClick={onClose}
      />

      {/* Side Modal */}
      <div className="fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-background border-l border-border z-50 overflow-y-auto">
        <DAGContext.Provider
          value={{
            refresh: refreshFn,
            fileName: fileName || '',
            name: data.dag?.name || '',
          }}
        >
          <RootDAGRunContext.Provider
            value={{
              data: currentDAGRun,
              setData: (status: components['schemas']['DAGRunDetails']) => {
                setCurrentDAGRun(status);
              },
            }}
          >
            <div className="p-6 w-full flex flex-col h-full dag-modal-content">
              <div className="flex justify-between items-center mb-4">
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
                    variant="outline"
                    size="icon"
                    onClick={handleFullscreenClick}
                    title="Open in fullscreen (F) - Cmd/Ctrl+Click to open in new tab"
                    className="relative group"
                  >
                    <Maximize2 className="h-4 w-4" />
                    <span className="absolute -bottom-1 -right-1 bg-muted text-muted-foreground text-xs font-medium px-1 rounded-sm border opacity-0 group-hover:opacity-100 transition-opacity">
                      F
                    </span>
                  </Button>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={onClose}
                    title="Close (Esc)"
                    className="relative group"
                  >
                    <X className="h-4 w-4" />
                    <span className="absolute -bottom-1 -right-1 bg-muted text-muted-foreground text-xs font-medium px-1 rounded-sm border opacity-0 group-hover:opacity-100 transition-opacity">
                      Esc
                    </span>
                  </Button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto overflow-x-hidden pr-4">
                {data.dag && (
                  <DAGDetailsContent
                    fileName={fileName}
                    dag={data.dag}
                    currentDAGRun={data.latestDAGRun}
                    refreshFn={refreshFn}
                    formatDuration={formatDuration}
                    activeTab={activeTab}
                    onTabChange={setActiveTab}
                    dagRunId={dagRunId}
                    stepName={stepName}
                    isModal={true}
                    navigateToStatusTab={navigateToStatusTab}
                    localDags={data?.localDags}
                  />
                )}
              </div>
            </div>
          </RootDAGRunContext.Provider>
        </DAGContext.Provider>
      </div>
    </>
  );
};

export default DAGDetailsModal;
