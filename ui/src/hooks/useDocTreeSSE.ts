import { components } from '../api/v1/schema';
import { buildSSEEndpoint, SSEState, useSSE } from './useSSE';

type DocListResponse = components['schemas']['DocListResponse'];

export function useDocTreeSSE(
  enabled: boolean = true
): SSEState<DocListResponse> {
  const endpoint = buildSSEEndpoint('/events/docs-tree', { perPage: 200 });
  return useSSE<DocListResponse>(endpoint, enabled);
}
