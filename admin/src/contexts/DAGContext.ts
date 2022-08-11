import React from 'react';
import { GetDAGResponse } from '../api/DAG';
import { DetailTabId } from '../models';

export const DAGContext = React.createContext({
  refresh: () => {
    return;
  },
  data: null as GetDAGResponse | null,
  name: '',
  tab: DetailTabId.Status,
});
