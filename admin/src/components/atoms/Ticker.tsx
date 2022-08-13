import * as React from 'react';

interface Props {
  intervalMs: number;
  children: () => JSX.Element;
}
export default function Ticker({ intervalMs, children }: Props) {
  const [, update] = React.useState(0);
  React.useEffect(() => {
    const interval = setInterval(() => {
      update((v) => v + 1);
    }, intervalMs);
    return () => clearInterval(interval);
  }, []);
  return children?.();
}
