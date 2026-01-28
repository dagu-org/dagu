import React, { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2, Maximize2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { usePageContext } from '../../../../contexts/PageContext';
import { useQuery } from '../../../../hooks/api';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import DAGRunDetailsContent from './DAGRunDetailsContent';

type DAGRunDetailsModalProps = {
  name: string;
  dagRunId: string;
  isOpen: boolean;
  onClose: () => void;
};

type PreviousData = {
  name: string;
  dagRunId: string;
  dagRunDetails: components['schemas']['DAGRunDetails'];
};

function DAGRunDetailsModal({
  name,
  dagRunId,
  isOpen,
  onClose,
}: DAGRunDetailsModalProps): React.ReactElement | null {
  const navigate = useNavigate();
  const appBarContext = useContext(AppBarContext);
  const { setContext } = usePageContext();

  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(false);
  const previousDataRef = useRef<PreviousData | null>(null);

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
      previousDataRef.current = null;
    }, 150);
    return () => clearTimeout(timer);
  }, [isOpen]);

  const searchParams = new URLSearchParams(window.location.search);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;
  const canQuerySubDag = Boolean(subDAGRunId && parentDAGRunId && parentName);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    {
      params: {
        query: { remoteNode },
        path: {
          name: parentName,
          dagRunId: parentDAGRunId ?? '',
          subDAGRunId: subDAGRunId ?? '',
        },
      },
    },
    {
      refreshInterval: 2000,
      keepPreviousData: true,
      isPaused: () => !canQuerySubDag,
    }
  );

  const dagRunQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    {
      params: {
        query: { remoteNode },
        path: {
          name: name || '',
          dagRunId: dagRunId || 'latest',
        },
      },
    },
    {
      refreshInterval: 2000,
      keepPreviousData: true,
      isPaused: () => canQuerySubDag,
    }
  );

  const activeQuery = canQuerySubDag ? subDAGQuery : dagRunQuery;
  const { data, isLoading, isValidating, mutate } = activeQuery;

  const freshDetails = data?.dagRunDetails;
  const displayData = freshDetails ?? previousDataRef.current?.dagRunDetails;
  const displayName = freshDetails ? name : (previousDataRef.current?.name ?? name);
  const displayDagRunId = freshDetails ? dagRunId : (previousDataRef.current?.dagRunId ?? dagRunId);

  useEffect(() => {
    if (freshDetails) {
      previousDataRef.current = { name, dagRunId, dagRunDetails: freshDetails };
    }
  }, [freshDetails, name, dagRunId]);

  const isInitialLoading = isLoading && !displayData;
  const previousData = previousDataRef.current;
  const isTransitioning =
    isValidating &&
    previousData !== null &&
    (previousData.dagRunId !== dagRunId || previousData.name !== name);

  useEffect(() => {
    if (isOpen && name) {
      setContext({
        dagFile: name,
        dagRunId: dagRunId || undefined,
        dagRunName: name,
        source: 'dag-run-details-modal',
      });
    }
  }, [isOpen, name, dagRunId, setContext]);

  const refreshFn = useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = useCallback(
    (e?: React.MouseEvent): void => {
      const url = `/dag-runs/${name}/${dagRunId}`;

      if (e?.metaKey || e?.ctrlKey) {
        window.open(url, '_blank');
      } else {
        navigate(url);
      }
    },
    [name, dagRunId, navigate]
  );

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }

      if (event.key === 'Escape') {
        onClose();
      }

      if (event.key === 'f' || event.key === 'F') {
        handleFullscreenClick();
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
      <div className="fixed inset-0 h-screen w-screen z-40" onClick={onClose} />

      <div
        className={`fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-background border-l border-indigo-500/30 z-50 overflow-y-auto transition-all duration-150 ease-out ${modalVisibilityClass}`}
      >
        <DAGRunContext.Provider
          value={{
            refresh: refreshFn,
            name: name || '',
            dagRunId: dagRunId || '',
          }}
        >
          <div className="p-6 w-full flex flex-col h-full dagRun-modal-content">
            <div className="flex justify-between items-center mb-4">
              <p className="text-xs text-muted-foreground">
                Use{' '}
                <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
                  ↑
                </kbd>{' '}
                <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
                  ↓
                </kbd>{' '}
                to navigate histories
              </p>
              <div className="flex gap-2 items-center">
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

            <div className="flex-1 overflow-y-auto relative">
              {isTransitioning && (
                <div className="absolute top-2 right-2 z-10">
                  <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                </div>
              )}

              {isInitialLoading && (
                <div className="flex items-center justify-center h-full">
                  <LoadingIndicator />
                </div>
              )}
              {!isInitialLoading && displayData && (
                <DAGRunDetailsContent
                  name={displayName}
                  dagRun={displayData}
                  refreshFn={refreshFn}
                  dagRunId={displayDagRunId}
                />
              )}
            </div>
          </div>
        </DAGRunContext.Provider>
      </div>
    </>
  );
}

export default DAGRunDetailsModal;
