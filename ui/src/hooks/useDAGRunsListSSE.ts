import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface DAGRunsListSSEResponse {
  dagRuns: DAGRunSummary[];
}

interface DAGRunsListParams {
  status?: string;
  fromDate?: number;
  toDate?: number;
  name?: string;
  dagRunId?: string;
  tags?: string;
}

function buildEndpoint(params: DAGRunsListParams): string {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null) {
      searchParams.set(key, String(value));
    }
  });

  const queryString = searchParams.toString();
  return queryString ? `/events/dag-runs?${queryString}` : '/events/dag-runs';
}

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true
): SSEState<DAGRunsListSSEResponse> {
  const endpoint = buildEndpoint(params);
  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
