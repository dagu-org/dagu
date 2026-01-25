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
  const entries = Object.entries(params).filter(
    ([, value]) => value !== undefined && value !== null
  );

  if (entries.length === 0) {
    return '/events/dag-runs';
  }

  const searchParams = new URLSearchParams();
  for (const [key, value] of entries) {
    searchParams.set(key, String(value));
  }

  return `/events/dag-runs?${searchParams.toString()}`;
}

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true
): SSEState<DAGRunsListSSEResponse> {
  const endpoint = buildEndpoint(params);
  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
