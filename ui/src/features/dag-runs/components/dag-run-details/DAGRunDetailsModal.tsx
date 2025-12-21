import { Button } from '@/components/ui/button';
import { Loader2, Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
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

  // Track if animation has already played to prevent re-animation on content changes
  const hasAnimatedRef = React.useRef(false);

  // Keep previous data to prevent flickering during navigation
  const previousDataRef = React.useRef<{
    name: string;
    dagRunId: string;
    dagRunDetails: components['schemas']['DAGRunDetails'];
  } | null>(null);

  // Reset animation flag when modal closes
  React.useEffect(() => {
    if (!isOpen) {
      hasAnimatedRef.current = false;
      previousDataRef.current = null;
    } else {
      // Mark as animated after first render when open
      hasAnimatedRef.current = true;
    }
  }, [isOpen]);

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
  const { data, isLoading, isValidating, mutate } = useQuery(
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

  // Update previous data ref when we get new data
  React.useEffect(() => {
    if (data?.dagRunDetails) {
      previousDataRef.current = {
        name,
        dagRunId,
        dagRunDetails: data.dagRunDetails,
      };
    }
  }, [data, name, dagRunId]);

  // Use current data or fall back to previous data to prevent flickering
  const displayData = data?.dagRunDetails || previousDataRef.current?.dagRunDetails;
  const displayName = data?.dagRunDetails ? name : (previousDataRef.current?.name || name);
  const displayDagRunId = data?.dagRunDetails ? dagRunId : (previousDataRef.current?.dagRunId || dagRunId);

  // Show loading indicator only on very first load (no previous data at all)
  const isInitialLoading = isLoading && !displayData;
  // Show subtle loading indicator when switching between items
  const isTransitioning = isValidating && previousDataRef.current &&
    (previousDataRef.current.dagRunId !== dagRunId || previousDataRef.current.name !== name);

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

  // Track if this is the initial open to apply animation only once
  const shouldAnimate = !hasAnimatedRef.current;

  if (!isOpen) return null;

  return (
    <>
      {/* Backdrop */}
      <div
        className={`fixed inset-0 h-screen w-screen bg-black/20 z-40 ${shouldAnimate ? 'fade-in' : ''}`}
        onClick={onClose}
      />

      {/* Side Modal - keep structure consistent to prevent re-animation */}
      <div
        className={`fixed top-0 bottom-0 right-0 md:w-3/4 w-full h-screen bg-background border-l border-border z-50 overflow-y-auto ${shouldAnimate ? 'slide-in-from-right' : ''}`}
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

            <div className="flex-1 overflow-y-auto relative">
              {/* Subtle loading indicator when transitioning between items */}
              {isTransitioning && (
                <div className="absolute top-2 right-2 z-10">
                  <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                </div>
              )}

              {isInitialLoading ? (
                <div className="flex items-center justify-center h-full">
                  <LoadingIndicator />
                </div>
              ) : displayData ? (
                <DAGRunDetailsContent
                  name={displayName}
                  dagRun={displayData}
                  refreshFn={refreshFn}
                  dagRunId={displayDagRunId}
                />
              ) : null}
            </div>
          </div>
        </DAGRunContext.Provider>
      </div>
    </>
  );
};

export default DAGRunDetailsModal;
