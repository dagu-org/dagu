import { components } from '../api/v1/schema';
import { buildSSEEndpoint, SSEState, useSSE } from './useSSE';

type DAGFile = components['schemas']['DAGFile'];
type Pagination = components['schemas']['Pagination'];

interface DAGsListSSEResponse {
  dags: DAGFile[];
  errors: string[];
  pagination: Pagination;
}

interface DAGsListParams {
  page?: number;
  perPage?: number;
  name?: string;
  tags?: string;
  sort?: string;
  order?: string;
}

export function useDAGsListSSE(
  params: DAGsListParams = {},
  enabled: boolean = true
): SSEState<DAGsListSSEResponse> {
  const endpoint = buildSSEEndpoint('/events/dags', params);
  return useSSE<DAGsListSSEResponse>(endpoint, enabled);
}

export type { DAGsListParams, DAGsListSSEResponse };
