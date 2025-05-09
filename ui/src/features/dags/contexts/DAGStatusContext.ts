import React from 'react';
import { components } from '../../../api/v2/schema';

type WorkflowDetailsContextType = {
  data: components['schemas']['WorkflowDetails'] | undefined;
  setData(workflowDetails: components['schemas']['WorkflowDetails']): void;
};

export const WorkflowDetailsContext =
  React.createContext<WorkflowDetailsContextType>({
    data: undefined as components['schemas']['WorkflowDetails'] | undefined,
    setData: () => {
      return;
    },
  });
