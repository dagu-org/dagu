import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import dayjs from '../../../../lib/dayjs';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { RootDAGRunContext } from '../../contexts/RootDAGRunContext';
import DAGDetailsContent from './DAGDetailsContent';

type DAGDetailsPanelProps = {
  fileName: string;
  onClose: () => void;
  onNavigate?: (direction: 'up' | 'down') => void;
};

const DAGDetailsPanel: React.FC<DAGDetailsPanelProps> = ({
  fileName,
  onClose,
  onNavigate,
}) => {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);
  const [currentDAGRun, setCurrentDAGRun] = React.useState<
    components['schemas']['DAGRunDetails'] | undefined
  >();
  const [activeTab, setActiveTab] = React.useState('status');
  const [dagRunId] = React.useState<string>('latest');
  const [stepName] = React.useState<string | null>(null);

  const navigateToStatusTab = React.useCallback(() => {
    setActiveTab('status');
  }, []);

  const [notFound, setNotFound] = React.useState(false);

  const { data, error, mutate } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileName: fileName || '',
        },
      },
    },
    {
      refreshInterval: notFound ? 0 : 2000, // Stop polling if DAG not found
      keepPreviousData: true,
    }
  );

  // Detect if DAG was deleted (404 error)
  React.useEffect(() => {
    if (error) {
      setNotFound(true);
    } else if (data) {
      setNotFound(false);
    }
  }, [error, data]);

  // Reset notFound state when fileName changes
  React.useEffect(() => {
    setNotFound(false);
  }, [fileName]);

  // Keep track of last valid data to prevent flickering
  const [lastValidData, setLastValidData] = React.useState(data);
  React.useEffect(() => {
    if (data) {
      setLastValidData(data);
    }
  }, [data]);
  const displayData = data || lastValidData;

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = (e?: React.MouseEvent) => {
    let url = `/dags/${fileName}`;
    if (activeTab !== 'status') {
      url = `${url}/${activeTab}`;
    }

    if (e && (e.metaKey || e.ctrlKey)) {
      window.open(url, '_blank');
    } else {
      navigate(url);
    }
  };

  React.useEffect(() => {
    if (displayData) {
      setCurrentDAGRun(displayData.latestDAGRun);
    }
  }, [displayData]);

  // Reset active tab when fileName changes
  React.useEffect(() => {
    setActiveTab('status');
  }, [fileName]);

  // Keyboard shortcuts
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }

      if (event.key === 'Escape') {
        onClose();
      }

      if (event.key === 'f' || event.key === 'F') {
        handleFullscreenClick();
      }

      if (event.key === 'ArrowDown' && onNavigate) {
        event.preventDefault();
        onNavigate('down');
      }

      if (event.key === 'ArrowUp' && onNavigate) {
        event.preventDefault();
        onNavigate('up');
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [onClose, onNavigate, handleFullscreenClick]);

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
  if (!displayData || !displayData.latestDAGRun) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingIndicator />
      </div>
    );
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileName: fileName || '',
        name: displayData.dag?.name || '',
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
        <div className="pl-2 pt-2 w-full flex flex-col h-full overflow-hidden">
          <div className="flex justify-between items-center mb-2 flex-shrink-0">
            <p className="text-xs text-muted-foreground">
              Use{' '}
              <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
                ↑
              </kbd>{' '}
              <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
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

          <div className="flex-1 overflow-y-auto min-h-0">
            {displayData.dag && (
              <DAGDetailsContent
                fileName={fileName}
                dag={displayData.dag}
                currentDAGRun={displayData.latestDAGRun}
                refreshFn={refreshFn}
                formatDuration={formatDuration}
                activeTab={activeTab}
                onTabChange={setActiveTab}
                dagRunId={dagRunId}
                stepName={stepName}
                isModal={true}
                navigateToStatusTab={navigateToStatusTab}
              />
            )}
          </div>
        </div>
      </RootDAGRunContext.Provider>
    </DAGContext.Provider>
  );
};

export default DAGDetailsPanel;
