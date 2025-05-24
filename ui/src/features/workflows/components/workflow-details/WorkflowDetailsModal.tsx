import { Button } from '@/components/ui/button';
import { Maximize2, X } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { WorkflowContext } from '../../contexts/WorkflowContext';
import { WorkflowActions } from '../common';
import WorkflowDetailsContent from './WorkflowDetailsContent';

type WorkflowDetailsModalProps = {
  name: string;
  workflowId: string;
  isOpen: boolean;
  onClose: () => void;
};

const WorkflowDetailsModal: React.FC<WorkflowDetailsModalProps> = ({
  name,
  workflowId,
  isOpen,
  onClose,
}) => {
  const navigate = useNavigate();
  const appBarContext = React.useContext(AppBarContext);

  // Check for child workflow ID in URL search params
  const searchParams = new URLSearchParams(window.location.search);
  const childWorkflowId = searchParams.get('childWorkflowId');
  const parentWorkflowId = searchParams.get('workflowId');
  const parentName = searchParams.get('workflowName') || name;

  // Determine the API endpoint based on whether this is a child workflow
  const endpoint = childWorkflowId
    ? '/workflows/{name}/{workflowId}/children/{childWorkflowId}'
    : '/workflows/{name}/{workflowId}';

  // Fetch workflow details
  const { data, isLoading, mutate } = useQuery(
    endpoint,
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: childWorkflowId
          ? {
              name: parentName || '',
              workflowId: parentWorkflowId || '',
              childWorkflowId: childWorkflowId,
            }
          : {
              name: name || '',
              workflowId: workflowId || 'latest',
            },
      },
    },
    { refreshInterval: 2000 }
  );

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate]);

  const handleFullscreenClick = (e?: React.MouseEvent) => {
    const url = `/workflows/${name}/${workflowId}`;

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
        <WorkflowContext.Provider
          value={{
            refresh: refreshFn,
            name: name || '',
            workflowId: workflowId || '',
          }}
        >
          <div className="p-6 w-full flex flex-col h-full workflow-modal-content">
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
                {data.workflowDetails && (
                  <WorkflowActions
                    workflow={data.workflowDetails}
                    name={data.workflowDetails.name}
                    refresh={refreshFn}
                    displayMode="compact"
                    isRootLevel={data.workflowDetails?.rootWorkflowId === data.workflowDetails?.workflowId}
                  />
                )}
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
              <WorkflowDetailsContent
                name={name}
                workflow={data.workflowDetails}
                refreshFn={refreshFn}
                workflowId={workflowId}
              />
            </div>
          </div>
        </WorkflowContext.Provider>
      </div>
    </>
  );
};

export default WorkflowDetailsModal;
