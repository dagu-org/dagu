/**
 * CreateDAGButton component provides a button to create a new DAG.
 *
 * @module features/dags/components/common
 */
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import React from 'react';
import { DAGNameInputModal } from '../../../../components/DAGNameInputModal';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';

/**
 * CreateDAGButton displays a button that opens a prompt to create a new DAG
 * and redirects to the DAG specification page after creation
 */
function CreateDAGButton() {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const config = useConfig();
  const [error, setError] = React.useState<string | null>(null);
  const [isOpen, setIsOpen] = React.useState(false);
  const [isLoading, setIsLoading] = React.useState(false);

  if (!config.permissions.writeDags) {
    return null;
  }

  const handleClose = () => {
    setIsOpen(false);
    setError(null);
  };

  const handleSubmit = async (name: string) => {
    setIsLoading(true);
    setError(null);

    try {
      const { error } = await client.POST('/dags', {
        params: {
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          name,
        },
      });

      if (error) {
        setError(error.message || 'An error occurred');
        setIsLoading(false);
        return;
      }

      // Success - close modal and redirect
      setIsOpen(false);

      // Redirect to the DAG specification page
      const basePath = window.location.pathname.split('/dags')[0] || '';
      window.location.href = `${basePath}/dags/${name}/spec`;
    } catch {
      setError('An unexpected error occurred');
      setIsLoading(false);
    }
  };

  return (
    <>
      <Button aria-label="Create new DAG" onClick={() => setIsOpen(true)}>
        <Plus className="h-4 w-4" />
        New
      </Button>

      <DAGNameInputModal
        isOpen={isOpen}
        onClose={handleClose}
        onSubmit={handleSubmit}
        mode="create"
        isLoading={isLoading}
        externalError={error}
      />
    </>
  );
}

export default CreateDAGButton;
