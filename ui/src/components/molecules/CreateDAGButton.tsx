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
        const resp = await fetch(`${getConfig().apiURL}/dags`, {
          method: 'POST',
          mode: 'cors',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            action: 'new',
            value: name,
          }),
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
