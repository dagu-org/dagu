import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

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

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true
) {
  const searchParams = new URLSearchParams();
  if (params.status) searchParams.set('status', params.status);
  if (params.fromDate) searchParams.set('fromDate', String(params.fromDate));
  if (params.toDate) searchParams.set('toDate', String(params.toDate));
  if (params.name) searchParams.set('name', params.name);
  if (params.dagRunId) searchParams.set('dagRunId', params.dagRunId);
  if (params.tags) searchParams.set('tags', params.tags);

  const queryString = searchParams.toString();
  const endpoint = queryString ? `/events/dag-runs?${queryString}` : '/events/dag-runs';

  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
