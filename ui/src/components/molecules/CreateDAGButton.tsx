import { Button } from '@mui/material';
import React from 'react';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useClient } from '../../hooks/api';

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
        const name = window.prompt('Please input the new DAG name', '');
        if (name === null) {
          return;
        }
        if (name === '') {
          alert('File name cannot be empty');
          return;
        }
        if (name.indexOf(' ') != -1) {
          alert('File name cannot contain space');
          return;
        }
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
        window.location.href = `${getConfig().basePath}/dags/${name}/spec`;
      }}
    >
      New
    </Button>
  );
}
export default CreateDAGButton;
