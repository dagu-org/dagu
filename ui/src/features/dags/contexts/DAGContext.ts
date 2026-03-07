import React from 'react';

export const DAGContext = React.createContext<{
  refresh: () => void;
  name: string;
  fileName: string;
  forceEnqueue: boolean;
  onEnqueue?: (params: string, dagRunId?: string, immediate?: boolean) => void | Promise<void>;
}>({
  refresh: () => {
    return;
  },
  name: '',
  fileName: '',
  forceEnqueue: false,
});
