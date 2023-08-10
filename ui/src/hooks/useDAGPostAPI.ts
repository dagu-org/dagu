import React from 'react';

type Options = {
  name: string;
  requestId?: string;
  onSuccess?: () => void;
  onFailed?: () => void;
};

export function useDAGPostAPI(opts: Options) {
  const doPost = React.useCallback(
    async (action: string, step?: string) => {
      const form = new FormData();
      form.set('action', action);
      if (opts.requestId) {
        form.set('request-id', opts.requestId);
      }
      if (step) {
        form.set('step', step);
      }
      const url = `${API_URL}/dags/${opts.name}`;
      const ret = await fetch(url, {
        method: 'POST',
        mode: 'cors',
        body: form,
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
