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

type DAGRunDetailsModalProps = {
  name: string;
  dagRunId: string;
  isOpen: boolean;
  onClose: () => void;
};

const DAGRunDetailsModal: React.FC<DAGRunDetailsModalProps> = ({
  name,
  dagRunId,
  isOpen,
  onClose,
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
  const { data, isLoading, mutate } = useQuery(
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
    { refreshInterval: 2000 }
  );

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = (e?: React.MouseEvent) => {
    const url = `/dag-runs/${name}/${dagRunId}`;

    // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, open in new tab
    if (e && (e.metaKey || e.ctrlKey)) {
      window.open(url, '_blank');
    } else {
      navigate(url);
    }
  };

  // Add keyboard shortcuts
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Ignore shortcuts when user is editing text (typing in inputs, textareas, editors, etc.)
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }

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

  if (!isOpen) return null;

  if (isLoading || !data) {
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
                <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
                  ↑
                </kbd>{' '}
                <kbd className="px-1 py-0.5 bg-muted rounded text-[10px] font-mono">
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
                  <span className="absolute -bottom-1 -right-1 bg-muted text-muted-foreground text-[10px] font-medium px-1 rounded-sm border opacity-0 group-hover:opacity-100 transition-opacity">
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
                  <span className="absolute -bottom-1 -right-1 bg-muted text-muted-foreground text-[10px] font-medium px-1 rounded-sm border opacity-0 group-hover:opacity-100 transition-opacity">
                    Esc
                  </span>
                </Button>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto">
              <DAGRunDetailsContent
                name={name}
                dagRun={data.dagRunDetails}
                refreshFn={refreshFn}
                dagRunId={dagRunId}
              />
            </div>
          </div>
        </DAGRunContext.Provider>
      </div>
    </>
  );
};

export default DAGRunDetailsModal;
