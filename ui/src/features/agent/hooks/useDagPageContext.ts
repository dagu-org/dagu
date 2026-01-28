import { useOptionalPageContext } from '@/contexts/PageContext';
import { DAGContext } from '../types';

/**
 * Hook to get the current page's DAG context for the agent chat.
 * Reads from the global PageContext which is set by pages/modals
 * when they display DAG or DAG-run content.
 */
export function useDagPageContext(): DAGContext | null {
  const pageContext = useOptionalPageContext();

  if (!pageContext?.context?.dagFile) {
    return null;
  }

  return {
    dag_file: pageContext.context.dagFile,
    dag_run_id: pageContext.context.dagRunId,
  };
}
