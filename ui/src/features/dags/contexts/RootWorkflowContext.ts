import React from 'react';
import { components } from '../../../api/v2/schema';

type RootWorkflowContextType = {
  data: components['schemas']['WorkflowDetails'] | undefined;
  setData(workflowDetails: components['schemas']['WorkflowDetails']): void;
};

export const RootWorkflowContext = React.createContext<RootWorkflowContextType>(
  {
    data: undefined as components['schemas']['WorkflowDetails'] | undefined,
    setData: () => {
      return;
    },
  }
);
