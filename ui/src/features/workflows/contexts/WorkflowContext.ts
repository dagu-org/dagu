import { createContext } from 'react';

interface WorkflowContextType {
  refresh: () => void;
  name: string;
  workflowId: string;
}

export const WorkflowContext = createContext<WorkflowContextType>({
  refresh: () => {},
  name: '',
  workflowId: '',
});
