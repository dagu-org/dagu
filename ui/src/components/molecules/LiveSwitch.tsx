import { Switch } from '@mui/material';
import React from 'react';
import { components } from '../../api/v2/schema';
import { useClient, useMutate } from '../../hooks/api';

type Props = {
  inputProps?: React.HTMLProps<HTMLInputElement>;
  dag: components['schemas']['DAGFile'];
  refresh?: () => void;
};

function LiveSwitch({ dag, refresh, inputProps }: Props) {
  const client = useClient();
  const mutate = useMutate();
  const [checked, setChecked] = React.useState(!dag.suspended);
  const onSubmit = React.useCallback(
    async (suspend: boolean) => {
      const { error } = await client.POST('/dags/{fileId}/suspend', {
        params: {
          path: {
            fileId: dag.fileId,
          },
        },
        body: {
          suspend,
        },
      });
      if (error) {
        alert(error.message || 'Error occurred');
        return;
      }
      mutate(['/dags/{fileId}']);
    },
    [refresh]
  );

  const onChange = React.useCallback(() => {
    const enabled = !checked;
    setChecked(enabled);
    onSubmit(!enabled);
  }, [dag, checked]);
  return (
    <Switch checked={checked} onChange={onChange} inputProps={inputProps} />
  );
}
export default LiveSwitch;
