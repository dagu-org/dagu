import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import { shouldIgnoreKeyboardShortcuts } from '../../../../lib/keyboard-shortcuts';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGRunContext } from '../../contexts/DAGRunContext';
import DAGRunDetailsContent from './DAGRunDetailsContent';

type DAGRunDetailsPanelProps = {
  name: string;
  dagRunId: string;
  onClose: () => void;
  onNavigate?: (direction: 'up' | 'down') => void;
};

const DAGRunDetailsPanel: React.FC<DAGRunDetailsPanelProps> = ({
  name,
  dagRunId,
  onClose,
  onNavigate,
}) => {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);

  // Check for sub DAG-run ID in URL search params
  const searchParams = new URLSearchParams(window.location.search);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  // Determine the API endpoint based on whether this is a sub DAG-run
  const endpoint = subDAGRunId
    ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}'
    : '/dag-runs/{name}/{dagRunId}';

  // Fetch DAG-run details
  const { data, mutate } = useQuery(
    endpoint,
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: subDAGRunId
          ? {
              name: parentName || '',
              dagRunId: parentDAGRunId || '',
              subDAGRunId: subDAGRunId,
            }
          : {
              name: name || '',
              dagRunId: dagRunId || 'latest',
            },
      },
    },
    { refreshInterval: 2000, keepPreviousData: true }
  );

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
    const url = `/dag-runs/${name}/${dagRunId}`;

    if (e && (e.metaKey || e.ctrlKey)) {
      window.open(url, '_blank');
    } else {
      navigate(url);
    }
  };

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
            <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
              ↑
            </kbd>{' '}
            <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
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
