import React from 'react';
import { Status } from '../models';

export const DAGStatusContext = React.createContext({
  data: undefined as Status | undefined,
  setData: (_: Status) => {
    return;
  },
});
