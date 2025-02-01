import React from 'react';
import { Status } from '../models';

type DAGStatusContextType = {
  data: Status | undefined;
  setData(val: Status): void;
};

export const DAGStatusContext = React.createContext<DAGStatusContextType>({
  data: undefined as Status | undefined,
  setData: () => {
    return;
  },
});
