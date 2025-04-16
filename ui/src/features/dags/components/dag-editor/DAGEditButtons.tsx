/**
 * DAGEditButtons component provides buttons for renaming and deleting a DAG.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { Button, Stack } from '@mui/material';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient } from '../../../../hooks/api';

/**
 * Props for the DAGEditButtons component
 */
type Props = {
  /** DAG file ID */
  fileId: string;
};

/**
 * DAGEditButtons provides buttons for renaming and deleting a DAG
 */
function DAGEditButtons({ fileId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  return (
    <Stack direction="row" spacing={1}>
      <Button
        onClick={async () => {
          const newFileId = window.prompt(
            'Please input the new DAG file ID',
            ''
          );
          if (!newFileId) {
            return;
          }
          if (newFileId.indexOf(' ') != -1) {
            alert('DAG file ID cannot contain space');
            return;
          }
          const { error } = await client.POST('/dags/{fileId}/rename', {
            params: {
              path: {
                fileId: fileId,
              },
            },
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
            body: {
              newFileId: newFileId,
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          // Redirect to the new DAG page
          const basePath = window.location.pathname.split('/dags')[0] || '';
          window.location.href = `${basePath}/dags/${newFileId}`;
        }}
      >
        Rename
      </Button>
      <Button
        onClick={async () => {
          if (!confirm('Are you sure to delete the DAG?')) {
            return;
          }
          const { error } = await client.DELETE('/dags/{fileId}', {
            params: {
              path: {
                fileId: fileId,
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
        Delete
      </Button>
    </Stack>
  );
}

export default DAGEditButtons;
