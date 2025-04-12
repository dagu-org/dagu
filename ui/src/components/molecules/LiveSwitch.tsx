import { Switch } from '@mui/material';
import React from 'react';
import { AppBarContext } from '../../contexts/AppBarContext';
import { components } from '../../api/v2/schema';

type Props = {
  inputProps?: React.HTMLProps<HTMLInputElement>;
  dag: components['schemas']['DAGFile'];
  refresh?: () => void;
};

function LiveSwitch({ dag, refresh, inputProps }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [checked, setChecked] = React.useState(!dag.suspended);
  const onSubmit = React.useCallback(
    async (params: { name: string; action: string; value: string }) => {
      const url = `${getConfig().apiURL}/dags/${params.name}?remoteNode=${
        appBarContext.selectedRemoteNode || 'local'
      }`;
      const ret = await fetch(url, {
        method: 'POST',
        mode: 'cors',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          action: params.action,
          value: params.value,
        }),
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
      name: dag.dag.name,
      action: 'suspend',
      value: enabled ? 'false' : 'true',
    });
  }, [dag, checked]);
  return (
    <Switch checked={checked} onChange={onChange} inputProps={inputProps} />
  );
}
export default LiveSwitch;
