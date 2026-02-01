import { useOptionalPageContext } from '@/contexts/PageContext';
import { DAGContext } from '../types';

/**
 * Returns the current page's DAG context for agent chat, or null if not viewing a DAG.
 */
export function useDagPageContext(): DAGContext | null {
  const context = useOptionalPageContext()?.context;

  if (!context?.dagFile) {
    return null;
  }

  return {
    dag_file: context.dagFile,
    dag_run_id: context.dagRunId,
  };
}
