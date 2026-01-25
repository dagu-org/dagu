import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

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

const BASE_ENDPOINT = '/events/dags';

function buildEndpoint(params: DAGsListParams): string {
  const searchParams = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    if (value != null) {
      searchParams.set(key, String(value));
    }
  }

  const queryString = searchParams.toString();
  return queryString ? `${BASE_ENDPOINT}?${queryString}` : BASE_ENDPOINT;
}

export function useDAGsListSSE(
  params: DAGsListParams = {},
  enabled: boolean = true
): SSEState<DAGsListSSEResponse> {
  const endpoint = buildEndpoint(params);
  return useSSE<DAGsListSSEResponse>(endpoint, enabled);
}

export type { DAGsListParams, DAGsListSSEResponse };
