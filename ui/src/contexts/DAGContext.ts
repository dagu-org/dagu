import React from 'react';

export const DAGContext = React.createContext({
  refresh: () => {
    return;
  },
  name: '',
});
