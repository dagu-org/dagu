import React, { useContext } from 'react';
import { useClient } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import DAGDetailsSidePanel from '@/features/dags/components/dag-details/DAGDetailsSidePanel';

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
    async (params: string, dagRunId?: string) => {
      const tags: string[] = [];
      if (selectedWorkspace) {
        const safeName = selectedWorkspace.replace(/[^a-zA-Z0-9_-]/g, '');
        if (safeName) {
          tags.push(`workspace=${safeName}`);
        }
      }

      const { data, error } = await client.POST('/dags/{fileName}/enqueue', {
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
        throw new Error(error.message || 'Failed to enqueue DAG execution.');
      }

      return data?.dagRunId;
    },
    [client, fileName, remoteNode, selectedWorkspace]
  );

  return (
    <DAGDetailsSidePanel
      fileName={fileName}
      isOpen={isOpen}
      onClose={onClose}
      initialTab="spec"
      renderInPortal={true}
      backdropVisibleClassName="bg-black/5"
      forceEnqueue={true}
      onEnqueue={handleEnqueue}
    />
  );
}
