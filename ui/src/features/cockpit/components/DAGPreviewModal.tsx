import React, { useContext, useMemo, useState } from 'react';
import { Play } from 'lucide-react';
import { useClient, useQuery } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import LoadingIndicator from '@/ui/LoadingIndicator';
import StartDAGModal from '@/features/dags/components/dag-execution/StartDAGModal';

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
  const [startModalOpen, setStartModalOpen] = useState(false);

  const { data, error, isLoading } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        path: { fileName },
        query: { remoteNode },
      },
    },
    {
      isPaused: () => !isOpen || !fileName,
      revalidateOnFocus: false,
      revalidateIfStale: false,
    }
  );

  const handleEnqueue = React.useCallback(
    async (params: string, dagRunId?: string): Promise<void> => {
      const tags: string[] = [];
      if (selectedWorkspace) {
        const safeName = selectedWorkspace.replace(/[^a-zA-Z0-9_-]/g, '');
        if (safeName) {
          tags.push(`workspace=${safeName}`);
        }
      }

      const { error } = await client.POST('/dags/{fileName}/enqueue', {
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

      setStartModalOpen(false);
      onClose();
    },
    [client, fileName, onClose, remoteNode, selectedWorkspace]
  );

  const spec = useMemo(() => data?.spec ?? '', [data?.spec]);
  const dagTitle = data?.dag?.name || fileName;
  const description =
    data?.dag?.description ||
    'Preview the workflow definition before enqueueing.';

  return (
    <>
      <Dialog open={isOpen} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
        <DialogContent className="max-w-4xl max-h-[85vh] overflow-hidden">
          <DialogHeader>
            <DialogTitle>{dagTitle}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>

          <div className="min-h-0 overflow-y-auto rounded-md border border-border bg-muted/20">
            {isLoading ? (
              <div className="flex items-center justify-center h-64">
                <LoadingIndicator />
              </div>
            ) : error ? (
              <div className="p-4 text-sm text-destructive">
                {(error as { message?: string } | undefined)?.message ||
                  'Failed to load DAG preview.'}
              </div>
            ) : (
              <pre className="p-4 text-xs leading-5 whitespace-pre-wrap break-words font-mono">
                {spec || '# No spec content available'}
              </pre>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={onClose}>
              Close
            </Button>
            <Button
              onClick={() => setStartModalOpen(true)}
              disabled={isLoading || !!error || !data?.dag}
            >
              <Play className="h-4 w-4 mr-2" />
              Enqueue
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <StartDAGModal
        dag={data?.dag}
        visible={startModalOpen}
        loading={isLoading}
        loadError={
          error
            ? (error as { message?: string } | undefined)?.message ||
              'Failed to load DAG details for execution.'
            : undefined
        }
        dismissModal={() => setStartModalOpen(false)}
        onSubmit={handleEnqueue}
        action="enqueue"
      />
    </>
  );
}
