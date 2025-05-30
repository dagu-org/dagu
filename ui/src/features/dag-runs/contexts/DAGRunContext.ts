import { createContext } from 'react';

interface DAGRunContextType {
  refresh: () => void;
  name: string;
  dagRunId: string;
}

export const DAGRunContext = createContext<DAGRunContextType>({
  refresh: () => {},
  name: '',
  dagRunId: '',
});
