import { useMemo } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { DAGContext } from '../types';

/**
 * Hook to extract DAG context from the current URL.
 * Returns the dag_file and dag_run_id if we're on a DAG page.
 */
export function useDagPageContext(): DAGContext | null {
  const params = useParams<{ fileName?: string }>();
  const [searchParams] = useSearchParams();

  return useMemo(() => {
    if (!params.fileName) {
      return null;
    }

    const dagRunId = searchParams.get('dagRunId') || undefined;

    return {
      dag_file: params.fileName,
      dag_run_id: dagRunId,
    };
  }, [params.fileName, searchParams]);
}
