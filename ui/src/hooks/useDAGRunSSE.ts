import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

interface DAGRunSSEResponse {
  dagRunDetails: DAGRunDetails;
}

export function useDAGRunSSE(
  name: string,
  dagRunId: string,
  enabled: boolean = true
) {
  const endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}`;
  return useSSE<DAGRunSSEResponse>(endpoint, enabled);
}
