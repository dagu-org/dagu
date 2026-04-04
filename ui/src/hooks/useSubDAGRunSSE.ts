import { components } from '../api/v1/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

interface SubDAGRunSSEResponse {
  dagRunDetails: DAGRunDetails;
}

export function useSubDAGRunSSE(
  name: string,
  dagRunId: string,
  subDAGRunId: string,
  enabled: boolean = true
): SSEState<SubDAGRunSSEResponse> {
  const endpoint =
    `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}` +
    `/sub-dag-runs/${encodeURIComponent(subDAGRunId)}`;
  return useSSE<SubDAGRunSSEResponse>(endpoint, enabled);
}

export type { SubDAGRunSSEResponse };
