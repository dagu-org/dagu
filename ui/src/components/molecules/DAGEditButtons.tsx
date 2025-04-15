import React from 'react';
import { Button, Stack } from '@mui/material';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useClient } from '../../hooks/api';

type Props = {
  location: string;
};

function DAGEditButtons({ location }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  return (
    <Stack direction="row" spacing={1}>
      <Button
        onClick={async () => {
          const newLocation = window.prompt(
            'Please input the new DAG name',
            ''
          );
          if (!newLocation) {
            return;
          }
          if (newLocation.indexOf(' ') != -1) {
            alert('DAG name cannot contain space');
            return;
          }
          const { error } = await client.POST('/dags/{dagLocation}/move', {
            params: {
              path: {
                dagLocation: location,
              },
            },
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
            body: {
              newLocation: newLocation,
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          window.location.href = `${getConfig().basePath}/dags/${newLocation}`;
        }}
      >
        Rename
      </Button>
      <Button
        onClick={async () => {
          if (!confirm('Are you sure to delete the DAG?')) {
            return;
          }
          const { error } = await client.DELETE('/dags/{dagLocation}', {
            params: {
              path: {
                dagLocation: location,
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
          window.location.href = `${getConfig().basePath}/dags/`;
        }}
      >
        Delete
      </Button>
    </Stack>
  );
}

export default DAGEditButtons;
