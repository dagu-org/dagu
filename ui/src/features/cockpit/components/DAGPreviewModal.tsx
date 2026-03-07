import React, { useCallback, useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useClient } from '@/hooks/api';
import { useDAGSSE } from '@/hooks/useDAGSSE';
import { sseFallbackOptions, useSSECacheSync } from '@/hooks/useSSECacheSync';
import { AppBarContext } from '@/contexts/AppBarContext';
import { RootDAGRunContext } from '@/features/dags/contexts/RootDAGRunContext';
import DAGDetailsContent from '@/features/dags/components/dag-details/DAGDetailsContent';
import { shouldIgnoreKeyboardShortcuts } from '@/lib/keyboard-shortcuts';
import dayjs from '@/lib/dayjs';
import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import type { components } from '@/api/v1/schema';

interface DAGPreviewModalProps {
  fileName: string;
  isOpen: boolean;
  selectedWorkspace: string;
  onClose: () => void;
}

export function DAGPreviewModal({ fileName, isOpen, selectedWorkspace, onClose }: DAGPreviewModalProps): React.ReactElement | null {
  const navigate = useNavigate();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();

  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(false);
  const [activeTab, setActiveTab] = useState('spec');
  const [enqueuedRunId, setEnqueuedRunId] = useState<string | undefined>();

  useEffect(() => {
    if (isOpen) {
      setShouldRender(true);
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setIsVisible(true);
        });
      });
      return;
    }

    setIsVisible(false);
    const timer = setTimeout(() => {
      setShouldRender(false);
    }, 150);
    return () => clearTimeout(timer);
  }, [isOpen]);

  const [currentDAGRun, setCurrentDAGRun] = useState<
    components['schemas']['DAGRunDetails'] | undefined
  >();
  // SSE for real-time updates
  const sseResult = useDAGSSE(fileName, !!fileName);

  // Fetch DAG details
  const { data, mutate } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: { remoteNode },
        path: { fileName },
      },
    },
    {
      ...sseFallbackOptions(sseResult),
      isPaused: () => !fileName,
    },
  );
  useSSECacheSync(sseResult, mutate);

  // Fetch specific DAG run when enqueued
  const dagName = data?.dag?.name || '';
  const { data: enqueuedRunData } = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    {
      params: {
        path: { name: dagName, dagRunId: enqueuedRunId || '' },
        query: { remoteNode },
      },
    },
    {
      isPaused: () => !dagName || !enqueuedRunId,
      refreshInterval: 2000,
    },
  );

  const refreshFn = useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  // Sync latest DAG run
  useEffect(() => {
    if (enqueuedRunData?.dagRunDetails) {
      setCurrentDAGRun(enqueuedRunData.dagRunDetails);
    } else if (data) {
      setCurrentDAGRun(data.latestDAGRun);
    }
  }, [data, enqueuedRunData]);

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

  // Custom enqueue handler — uses file-based enqueue with tags parameter
  const handleEnqueue = useCallback(async (params: string, dagRunId?: string) => {
    const tags: string[] = [];
    if (selectedWorkspace) {
      const safeName = selectedWorkspace.replace(/[^a-zA-Z0-9_-]/g, '');
      if (safeName) {
        tags.push(`workspace=${safeName}`);
      }
    }

    const { data: enqueueData, error } = await client.POST('/dags/{fileName}/enqueue', {
      params: {
        path: { fileName },
        query: { remoteNode },
      },
      body: {
        params: params || undefined,
        dagRunId: dagRunId || undefined,
        tags: tags.length > 0 ? tags : undefined,
      },
    });

    if (error) {
      console.error('Failed to enqueue:', error);
      return;
    }

    if (enqueueData?.dagRunId) {
      setEnqueuedRunId(enqueueData.dagRunId);
    }
    setActiveTab('status');
  }, [selectedWorkspace, client, remoteNode, fileName]);

  // Fullscreen navigation
  const handleFullscreenClick = useCallback(
    (e?: React.MouseEvent) => {
      const url = `/dags/${fileName}/spec`;
      if (e?.metaKey || e?.ctrlKey) {
        window.open(url, '_blank', 'noopener,noreferrer');
      } else {
        navigate(url);
      }
    },
    [fileName, navigate]
  );

  // Keyboard shortcuts
  useEffect(() => {
    if (!isOpen) return;

    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) return;
      if (event.metaKey || event.ctrlKey) return;

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

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, handleFullscreenClick]);

  if (!shouldRender) {
    return null;
  }

  const modalVisibilityClass = isVisible
    ? 'translate-x-0 opacity-100'
    : 'translate-x-full opacity-0';

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 h-screen w-screen z-40"
        onClick={onClose}
      />

      {/* Side Panel */}
      <div className={`fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-background border-l border-border z-50 overflow-y-auto transition-all duration-150 ease-out ${modalVisibilityClass}`}>
        <RootDAGRunContext.Provider
          value={{
            data: currentDAGRun,
            setData: (status: components['schemas']['DAGRunDetails']) => {
              setCurrentDAGRun(status);
            },
          }}
        >
          <div className="p-6 w-full flex flex-col h-full dag-modal-content">
            {/* Toolbar */}
            <div className="flex justify-between items-center mb-4">
              <div />
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

            {/* Content */}
            <div className="flex-1 overflow-y-auto overflow-x-hidden pr-4">
              {data?.dag && (
                <DAGDetailsContent
                  fileName={fileName}
                  dag={data.dag}
                  currentDAGRun={currentDAGRun}
                  dagRunId={enqueuedRunId}
                  refreshFn={refreshFn}
                  formatDuration={formatDuration}
                  activeTab={activeTab}
                  onTabChange={setActiveTab}
                  isModal={true}
                  localDags={data?.localDags}
                  sseResult={sseResult}
                  onEnqueue={handleEnqueue}
                  forceEnqueue={true}
                />
              )}
            </div>
          </div>
        </RootDAGRunContext.Provider>
      </div>
    </>
  );
}
