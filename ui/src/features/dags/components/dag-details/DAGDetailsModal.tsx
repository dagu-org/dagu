import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import dayjs from '../../../../lib/dayjs';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { RunDetailsContext } from '../../contexts/DAGStatusContext';
import DAGDetailsContent from './DAGDetailsContent';

type DAGDetailsModalProps = {
  fileId: string;
  isOpen: boolean;
  onClose: () => void;
};

const DAGDetailsModal: React.FC<DAGDetailsModalProps> = ({
  fileId,
  isOpen,
  onClose,
}) => {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);
  const [currentRun, setCurrentRun] = React.useState<
    components['schemas']['RunDetails'] | undefined
  >();
  const [activeTab, setActiveTab] = React.useState('status');
  const [requestId, setRequestId] = React.useState<string>('latest');
  const [stepName, setStepName] = React.useState<string | null>(null);

  const { data, isLoading, mutate } = useQuery(
    '/dags/{fileId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: fileId || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = (e?: React.MouseEvent) => {
    // Determine the URL path based on the active tab
    let url = `/dags/${fileId}`;

    // Add the tab to the URL if it's not the default 'status' tab
    if (activeTab !== 'status') {
      url = `${url}/${activeTab}`;
    }

    // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, open in new tab
    if (e && (e.metaKey || e.ctrlKey)) {
      window.open(url, '_blank');
    } else {
      navigate(url);
    }
  };

  React.useEffect(() => {
    if (data) {
      setCurrentRun(data.latestRun);
    }
  }, [data]);

  // Add keyboard shortcuts
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Close modal with Escape key
      if (event.key === 'Escape') {
        onClose();
      }

      // Open in fullscreen with 'f' key
      if (event.key === 'f' || event.key === 'F') {
        handleFullscreenClick();
      }
    };

    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose, handleFullscreenClick]);

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

  if (!isOpen) return null;

  if (isLoading || !data || !data.latestRun) {
    return (
      <div className="fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-gray-100 border-l border-border shadow-xl z-50 flex items-center justify-center">
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
      <div className="fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-gray-100 border-l border-border shadow-xl z-50 overflow-y-auto">
        <DAGContext.Provider
          value={{
            refresh: refreshFn,
            fileId: fileId || '',
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
            <div className="p-6 w-full flex flex-col h-full dag-modal-content">
              <div className="flex justify-between items-center mb-4">
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
                    variant="outline"
                    size="icon"
                    onClick={handleFullscreenClick}
                    title="Open in fullscreen (F) - Cmd/Ctrl+Click to open in new tab"
                    className="relative group"
                  >
                    <Maximize2 className="h-4 w-4" />
                    <span className="absolute -bottom-1 -right-1 bg-primary text-primary-foreground text-[10px] font-medium px-1 rounded-sm opacity-0 group-hover:opacity-100 transition-opacity">
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
                    <span className="absolute -bottom-1 -right-1 bg-primary text-primary-foreground text-[10px] font-medium px-1 rounded-sm opacity-0 group-hover:opacity-100 transition-opacity">
                      Esc
                    </span>
                  </Button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto">
                {data.dag && (
                  <DAGDetailsContent
                    fileId={fileId}
                    dag={data.dag}
                    latestRun={data.latestRun}
                    refreshFn={refreshFn}
                    formatDuration={formatDuration}
                    activeTab={activeTab}
                    onTabChange={setActiveTab}
                    requestId={requestId}
                    stepName={stepName}
                    isModal={true}
                  />
                )}
              </div>
            </div>
          </RunDetailsContext.Provider>
        </DAGContext.Provider>
      </div>
    </>
  );
};

export default DAGDetailsModal;
