import React from "react";

type Options = {
  onFailed?: () => void;
};

export function useGetApi<T>(path: string, opts: Options) {
  const [data, setData] = React.useState<T | null>(null);
  const doGet = React.useCallback(async () => {
    const urlParams = new URLSearchParams(window.location.search);
    let url = `${API_URL}${path}?format=json`;
    const group = urlParams.get("group");
    if (group) {
      url += `&group=${encodeURI(group)}`;
    }
    const ret = await fetch(url, {
      method: "GET",
      cache: "no-store",
      mode: "cors",
      headers: {
        Accept: "application/json",
      },
    });
    if (ret.ok) {
      const body = await ret.json();
      setData(body);
    } else {
      const e = await ret.text();
      alert(e);
      opts.onFailed && opts.onFailed();
    }
  }, [path, opts]);

  return {
    data,
    doGet,
  };
}
