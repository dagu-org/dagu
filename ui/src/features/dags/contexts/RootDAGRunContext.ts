import React from 'react';
import { components } from '../../../api/v2/schema';

type RootDAGRunContextType = {
  data: components['schemas']['DAGRunDetails'] | undefined;
  setData(dagRunDetails: components['schemas']['DAGRunDetails']): void;
};

export const RootDAGRunContext = React.createContext<RootDAGRunContextType>(
  {
    data: undefined as components['schemas']['DAGRunDetails'] | undefined,
    setData: () => {
      return;
    },
  }
);
