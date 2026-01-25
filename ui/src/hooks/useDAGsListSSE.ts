import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

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
) {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null) {
      searchParams.set(key, String(value));
    }
  });

  const queryString = searchParams.toString();
  const endpoint = queryString ? `/events/dags?${queryString}` : '/events/dags';

  return useSSE<DAGsListSSEResponse>(endpoint, enabled);
}

export type { DAGsListParams, DAGsListSSEResponse };
