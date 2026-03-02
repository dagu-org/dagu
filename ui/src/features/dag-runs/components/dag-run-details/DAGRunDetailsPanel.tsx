import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import { useDAGRunSSE } from '../../../../hooks/useDAGRunSSE';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import DAGRunDetailsContent from './DAGRunDetailsContent';

type Props = {
  name: string;
  dagRunId: string;
  onClose: () => void;
  onNavigate?: (direction: 'up' | 'down') => void;
};

function DAGRunDetailsPanel({
  name,
  dagRunId,
  onClose,
  onNavigate,
}: Props): React.ReactElement {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);

  // Parse sub DAG-run params from URL
  const searchParams = new URLSearchParams(window.location.search);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;
  const isSubDAGRun = Boolean(subDAGRunId && parentDAGRunId && parentName);

  // SSE for real-time updates (disabled for sub-DAG runs)
  const sseResult = useDAGRunSSE(
    name || '',
    dagRunId || 'latest',
    !isSubDAGRun
  );

  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Sub-DAG query (only enabled for sub-DAG runs)
  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    {
      params: {
        query: { remoteNode },
        path: {
          name: parentName as string,
          dagRunId: parentDAGRunId as string,
          subDAGRunId: subDAGRunId as string,
        },
      },
    },
    { refreshInterval: 2000, keepPreviousData: true, isPaused: () => !isSubDAGRun }
  );

  // Regular DAG query (fallback when SSE is unavailable)
  const sseIsActive = sseResult.isConnected && !sseResult.shouldUseFallback;
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
      refreshInterval: sseIsActive ? 0 : 2000,
      keepPreviousData: true,
      isPaused: () => isSubDAGRun || sseIsActive,
    }
  );

  // Select data source: sub-DAG query, SSE data, or polling query
  const { mutate } = isSubDAGRun ? subDAGQuery : dagRunQuery;
  const data = isSubDAGRun
    ? subDAGQuery.data
    : sseResult.data || dagRunQuery.data;

  // Keep track of last valid data to prevent flickering
  const [lastValidData, setLastValidData] = React.useState(data);
  React.useEffect(() => {
    if (data) {
      setLastValidData(data);
    }
  }, [data]);

  React.useEffect(() => {
    setLastValidData(null);
  }, [remoteNode]);

  const displayData = data || lastValidData;

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = React.useCallback(
    (e?: React.MouseEvent) => {
      const url = `/dag-runs/${name}/${dagRunId}`;

      if (e && (e.metaKey || e.ctrlKey)) {
        window.open(url, '_blank');
      } else {
        navigate(url);
      }
    },
    [name, dagRunId, navigate]
  );

  // Keyboard shortcuts
  React.useEffect(() => {
    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
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
        case 'ArrowDown':
          if (onNavigate) {
            event.preventDefault();
            onNavigate('down');
          }
          break;
        case 'ArrowUp':
          if (onNavigate) {
            event.preventDefault();
            onNavigate('up');
          }
          break;
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [onClose, onNavigate, handleFullscreenClick]);

  // Only show loading on initial load, not when switching DAG runs
  if (!displayData) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingIndicator />
      </div>
    );
  }

  return (
    <DAGRunContext.Provider
      value={{
        refresh: refreshFn,
        name: name || '',
        dagRunId: dagRunId || '',
      }}
    >
      <div className="p-4 w-full flex flex-col h-full overflow-hidden">
        <div className="flex justify-between items-center mb-3 flex-shrink-0">
          <p className="text-xs text-muted-foreground">
            Use{' '}
            <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
              ↑
            </kbd>{' '}
            <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">
              ↓
            </kbd>{' '}
            to navigate runs
          </p>
          <div className="flex gap-2 items-center">
            <Button
              size="icon"
              onClick={handleFullscreenClick}
              title="Open in fullscreen (F) - Cmd/Ctrl+Click to open in new tab"
            >
              <Maximize2 className="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              onClick={onClose}
              title="Close (Esc)"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto min-h-0">
          <DAGRunDetailsContent
            name={name}
            dagRun={displayData.dagRunDetails}
            refreshFn={refreshFn}
            dagRunId={dagRunId}
          />
        </div>
      </div>
    </DAGRunContext.Provider>
  );
};

export default DAGRunDetailsPanel;
