import { Button } from '@mui/material';
import React from 'react';

function CreateDAGButton() {
  return (
    <Button
      variant="outlined"
      size="small"
      sx={{
        width: '100px',
      }}
      onClick={async () => {
        const name = window.prompt('Please input the new DAG name', '');
        if (name == '') {
          return;
        }
        if (name?.indexOf(' ') != -1) {
          alert('File name cannot contain space');
          return;
        }
        const formData = new FormData();
        formData.append('action', 'new');
        formData.append('value', name);
        const resp = await fetch(API_URL, {
          method: 'POST',
          mode: 'cors',
          headers: {
            Accept: 'application/json',
          },
          body: formData,
        });
        if (resp.ok) {
          window.location.href = `/dags/${name.replace(/.yaml$/, '')}/spec`;
        } else {
          const e = await resp.text();
          alert(e);
        }
      }}
    >
      New
    </Button>
  );
}
export default CreateDAGButton;
