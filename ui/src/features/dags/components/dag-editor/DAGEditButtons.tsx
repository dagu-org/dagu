/**
 * DAGEditButtons component provides buttons for renaming and deleting a DAG.
 *
 * @module features/dags/components/dag-editor
 */
import { Button } from '@/components/ui/button';
import { PencilLine, Trash2 } from 'lucide-react';
import React from 'react';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';
import { DAGNameInputModal } from '../../../../components/DAGNameInputModal';

/**
 * Props for the DAGEditButtons component
 */
type Props = {
  /** DAG file name */
  fileName: string;
};

/**
 * DAGEditButtons provides buttons for renaming and deleting a DAG
 */
function DAGEditButtons({ fileName }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const config = useConfig();
  const [isRenameModalOpen, setIsRenameModalOpen] = React.useState(false);
  const [renameError, setRenameError] = React.useState<string | null>(null);
  const [isRenameLoading, setIsRenameLoading] = React.useState(false);

  if (!config.permissions.writeDags) {
    return null;
  }

  const handleRenameClose = () => {
    setIsRenameModalOpen(false);
    setRenameError(null);
  };

  const handleRenameSubmit = async (newFileName: string) => {
    setIsRenameLoading(true);
    setRenameError(null);

    try {
      const { error } = await client.POST('/dags/{fileName}/rename', {
        params: {
          path: {
            fileName: fileName,
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          newFileName: newFileName,
        },
      });
      
      if (error) {
        setRenameError(error.message || 'An error occurred');
        setIsRenameLoading(false);
        return;
      }
      
      // Success - close modal and redirect
      setIsRenameModalOpen(false);
      
      // Redirect to the new DAG page
      const basePath = window.location.pathname.split('/dags')[0] || '';
      window.location.href = `${basePath}/dags/${newFileName}`;
    } catch {
      setRenameError('An unexpected error occurred');
      setIsRenameLoading(false);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="sm"
        className="cursor-pointer"
        onClick={() => setIsRenameModalOpen(true)}
      >
        <PencilLine className="h-4 w-4 mr-2" />
        Rename
      </Button>

      <Button
        variant="outline"
        size="sm"
        className="text-red-600 hover:text-red-600 border-red-200 hover:border-red-300 hover:bg-red-50 dark:text-red-500 dark:hover:text-red-400 dark:border-red-900 dark:hover:border-red-800 dark:hover:bg-red-950 cursor-pointer"
        onClick={async () => {
          if (!confirm('Are you sure to delete the DAG?')) {
            return;
          }
          const { error } = await client.DELETE('/dags/{fileName}', {
            params: {
              path: {
                fileName: fileName,
              },
              query: {
                remoteNode: appBarContext.selectedRemoteNode || 'local',
              },
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          // Redirect to the DAGs list page
          const basePath = window.location.pathname.split('/dags')[0] || '';
          window.location.href = `${basePath}/dags/`;
        }}
      >
        <Trash2 className="h-4 w-4 mr-2" />
        Delete
      </Button>
      
      <DAGNameInputModal
        isOpen={isRenameModalOpen}
        onClose={handleRenameClose}
        onSubmit={handleRenameSubmit}
        mode="rename"
        initialValue={fileName}
        isLoading={isRenameLoading}
        externalError={renameError}
      />
    </div>
  );
}

export default DAGEditButtons;
