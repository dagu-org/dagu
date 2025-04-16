import React from 'react';
import { components } from '../../../api/v2/schema';

type RunDetailsContextType = {
  data: components['schemas']['RunDetails'] | undefined;
  setData(runDetails: components['schemas']['RunDetails']): void;
};

export const RunDetailsContext = React.createContext<RunDetailsContextType>({
  data: undefined as components['schemas']['RunDetails'] | undefined,
  setData: () => {
    return;
  },
});
