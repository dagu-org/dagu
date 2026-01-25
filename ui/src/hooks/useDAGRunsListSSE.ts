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

const BASE_ENDPOINT = '/events/dag-runs';

function buildEndpoint(params: DAGRunsListParams): string {
  const searchParams = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    if (value != null) {
      searchParams.set(key, String(value));
    }
  }

  const queryString = searchParams.toString();
  return queryString ? `${BASE_ENDPOINT}?${queryString}` : BASE_ENDPOINT;
}

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true
): SSEState<DAGRunsListSSEResponse> {
  const endpoint = buildEndpoint(params);
  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
