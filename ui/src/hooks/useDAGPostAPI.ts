import React from 'react';
import { AppBarContext } from '../contexts/AppBarContext';

type Options = {
  name: string;
  requestId?: string;
  onSuccess?: () => void;
  onFailed?: () => void;
};

export function useDAGPostAPI(opts: Options) {
  const appBarContext = React.useContext(AppBarContext);
  const doPost = React.useCallback(
    async (action: string, step?: string) => {
      const url = `${getConfig().apiURL}/dags/${opts.name}?remoteNode=${
        appBarContext.selectedRemoteNode || 'local'
      }`;
      const ret = await fetch(url, {
        method: 'POST',
        mode: 'cors',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          action: action,
          step: step,
          requestId: opts.requestId,
        }),
      });
      if (ret.ok) {
        opts?.onSuccess?.();
      } else {
        const e = await ret.text();
        alert(e);
        opts?.onFailed?.();
      }
    },
    [opts]
  );
  return {
    doPost,
  };
}
