import React from 'react';

export const DAGContext = React.createContext<{
  refresh: () => void;
  name: string;
  fileName: string;
  forceEnqueue?: boolean;
  autoOpenStartModal?: boolean;
  onEnqueue?: (
    params: string,
    dagRunId?: string,
    immediate?: boolean
  ) => string | void | Promise<string | void>;
}>({
  refresh: () => {
    return;
  },
  name: '',
  fileName: '',
  forceEnqueue: false,
  autoOpenStartModal: false,
});
