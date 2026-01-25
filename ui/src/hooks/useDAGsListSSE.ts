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

export function useDAGsListSSE(params: DAGsListParams = {}, enabled: boolean = true) {
  const searchParams = new URLSearchParams();
  if (params.page) searchParams.set('page', String(params.page));
  if (params.perPage) searchParams.set('perPage', String(params.perPage));
  if (params.name) searchParams.set('name', params.name);
  if (params.tags) searchParams.set('tags', params.tags);
  if (params.sort) searchParams.set('sort', params.sort);
  if (params.order) searchParams.set('order', params.order);

  const queryString = searchParams.toString();
  const endpoint = queryString ? `/events/dags?${queryString}` : '/events/dags';

  return useSSE<DAGsListSSEResponse>(endpoint, enabled);
}

export type { DAGsListParams, DAGsListSSEResponse };
