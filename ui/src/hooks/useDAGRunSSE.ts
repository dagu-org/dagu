import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

interface DAGRunSSEResponse {
  dagRunDetails: DAGRunDetails;
}

export function useDAGRunSSE(
  name: string,
  dagRunId: string,
  enabled: boolean = true
): SSEState<DAGRunSSEResponse> {
  const endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}`;
  return useSSE<DAGRunSSEResponse>(endpoint, enabled);
}

export type { DAGRunSSEResponse };
