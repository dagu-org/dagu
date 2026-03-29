import React, { useContext } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import DAGDetailsSidePanel from '@/features/dags/components/dag-details/DAGDetailsSidePanel';
import { submitDAGExecution } from '@/features/dags/lib/submitDAGExecution';

interface DAGPreviewModalProps {
  fileName: string;
  isOpen: boolean;
  selectedWorkspace: string;
  onClose: () => void;
}

export function DAGPreviewModal({
  fileName,
  isOpen,
  selectedWorkspace,
  onClose,
}: DAGPreviewModalProps): React.ReactElement | null {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const handleEnqueue = React.useCallback(
    async (params: string, dagRunId?: string): Promise<string | void> => {
      return submitDAGExecution({
        client,
        fileName,
        remoteNode,
        selectedWorkspace,
        params,
        dagRunId,
      });
    },
    [client, fileName, remoteNode, selectedWorkspace]
  );

  const toolbarHint = selectedWorkspace ? (
    <>
      Workspace:{' '}
      <span className="font-medium text-foreground">{selectedWorkspace}</span>
    </>
  ) : (
    'Template details'
  );

  return (
    <DAGDetailsSidePanel
      fileName={fileName}
      isOpen={isOpen}
      onClose={onClose}
      initialTab="status"
      toolbarHint={toolbarHint}
      renderInPortal={true}
      forceEnqueue={true}
      onEnqueue={handleEnqueue}
    />
  );
}
