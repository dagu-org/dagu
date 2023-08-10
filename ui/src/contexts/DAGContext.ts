import React from 'react';
import { GetDAGResponse } from '../models/api';

export const DAGContext = React.createContext({
  refresh: () => {
    return;
  },
  data: null as GetDAGResponse | null,
  name: '',
});
