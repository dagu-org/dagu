import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunDetails = components['schemas']['DAGRunDetails'];
type DAGGridItem = components['schemas']['DAGGridItem'];

interface DAGHistorySSEResponse {
  dagRuns: DAGRunDetails[];
  gridData: DAGGridItem[];
}

export function useDAGHistorySSE(
  fileName: string,
  enabled: boolean = true
): SSEState<DAGHistorySSEResponse> {
  const endpoint = `/events/dags/${encodeURIComponent(fileName)}/dag-runs`;
  return useSSE<DAGHistorySSEResponse>(endpoint, enabled);
}
