/**
 * CreateDAGButton component provides a button to create a new DAG.
 *
 * @module features/dags/components/common
 */
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import React from 'react';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient } from '../../../../hooks/api';

/**
 * CreateDAGButton displays a button that opens a prompt to create a new DAG
 * and redirects to the DAG specification page after creation
 */
function CreateDAGButton() {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();

  return (
    <Button
      aria-label="Create new DAG"
      className="flex items-center gap-1.5 bg-primary text-white font-medium px-3 py-1 text-sm rounded-md shadow-sm hover:bg-primary/90 focus:outline-none focus:ring-1 focus:ring-primary focus:ring-offset-1 transition cursor-pointer h-8"
      onClick={async () => {
        // Prompt for the new DAG name
        const name = window.prompt('Please input the new DAG name', '');
        if (name === null) {
          return;
        }

        // Validate the name
        if (name === '') {
          alert('File name cannot be empty');
          return;
        }
        if (name.indexOf(' ') != -1) {
          alert('File name cannot contain space');
          return;
        }

        // Create the DAG
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
          alert(error.message || 'An error occurred');
          return;
        }

        // Redirect to the DAG specification page
        // Assuming basePath is defined in the global config
        const basePath = window.location.pathname.split('/dags')[0] || '';
        window.location.href = `${basePath}/dags/${name}/spec`;
      }}
    >
      <Plus className="w-3.5 h-3.5" aria-hidden="true" />
      <span>New</span>
    </Button>
  );
}

export default CreateDAGButton;
