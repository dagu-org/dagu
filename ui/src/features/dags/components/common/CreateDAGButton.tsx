/**
 * CreateDAGButton component provides a button to create a new DAG.
 *
 * @module features/dags/components/common
 */
import { Button } from '@mui/material';
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
      variant="outlined"
      size="small"
      sx={{
        width: '100px',
      }}
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
      New
    </Button>
  );
}

export default CreateDAGButton;
