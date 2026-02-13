import { components } from '../api/v1/schema';
import { buildSSEEndpoint, SSEState, useSSE } from './useSSE';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface DAGRunsListSSEResponse {
  dagRuns: DAGRunSummary[];
}

interface DAGRunsListParams {
  status?: number;
  fromDate?: number;
  toDate?: number;
  name?: string;
  dagRunId?: string;
  tags?: string;
}

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true
): SSEState<DAGRunsListSSEResponse> {
  const endpoint = buildSSEEndpoint('/events/dag-runs', params);
  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
