import { Switch } from '@mui/material';
import React from 'react';
import { DAGStatus } from '../../models';

type Props = {
  inputProps?: React.HTMLProps<HTMLInputElement>;
  DAG: DAGStatus;
  refresh?: () => void;
};

function LiveSwitch({ DAG, refresh, inputProps }: Props) {
  const [checked, setChecked] = React.useState(!DAG.Suspended);
  const onSubmit = React.useCallback(
    async (params: { name: string; action: string; value: string }) => {
      const form = new FormData();
      form.set('action', params.action);
      form.set('value', params.value);
      const url = `${API_URL}/dags/${params.name}`;
      const ret = await fetch(url, {
        method: 'POST',
        mode: 'cors',
        body: form,
      });
      if (ret.ok) {
        if (refresh) {
          refresh();
        }
      } else {
        const e = await ret.text();
        alert(e);
      }
    },
    [refresh]
  );

  const onChange = React.useCallback(() => {
    const enabled = !checked;
    setChecked(enabled);
    onSubmit({
      name: DAG.DAG.Name,
      action: 'suspend',
      value: enabled ? 'false' : 'true',
    });
  }, [DAG, checked]);
  return (
    <Switch
      checked={checked}
      onChange={onChange}
      inputProps={inputProps}
    />
  );
}
export default LiveSwitch;
