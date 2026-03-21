import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import { whenEnabled } from '../../../../hooks/queryUtils';
import {
  liveFallbackOptions,
  useLiveConnection,
  useLiveDAGRuns,
} from '../../../../hooks/useAppLive';
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
  const liveEnabled = isSubDAGRun || Boolean(name && dagRunId);

  const liveState = useLiveConnection(liveEnabled);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const subDAGQueryEnabled = Boolean(isSubDAGRun);
  const dagRunQueryEnabled = !isSubDAGRun && Boolean(name && dagRunId);

  // Sub-DAG query (only enabled for sub-DAG runs)
  const subDAGQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}',
    whenEnabled(subDAGQueryEnabled, {
      params: {
        query: { remoteNode },
        path: {
          name: parentName as string,
          dagRunId: parentDAGRunId as string,
          subDAGRunId: subDAGRunId as string,
        },
      },
    }),
    liveFallbackOptions(liveState)
  );
  useLiveDAGRuns(subDAGQuery.mutate, subDAGQueryEnabled);

  // Regular DAG query — SWR is the single source of truth, refreshed by live invalidations
  const dagRunQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}',
    whenEnabled(dagRunQueryEnabled, {
      params: {
        query: { remoteNode },
        path: {
          name: name || '',
          dagRunId: dagRunId || 'latest',
        },
      },
    }),
    liveFallbackOptions(liveState)
  );
  useLiveDAGRuns(dagRunQuery.mutate, dagRunQueryEnabled);

  // Select data source: sub-DAG query or regular query
  const { mutate } = isSubDAGRun ? subDAGQuery : dagRunQuery;
  const data = isSubDAGRun ? subDAGQuery.data : dagRunQuery.data;

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
  if (!data) {
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
            dagRun={data.dagRunDetails}
            refreshFn={refreshFn}
            dagRunId={dagRunId}
          />
        </div>
      </div>
    </DAGRunContext.Provider>
  );
};

export default DAGRunDetailsPanel;
