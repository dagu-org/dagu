import { components } from '../api/v1/schema';
import { SSEState, useSSE } from './useSSE';

type DocResponse = components['schemas']['DocResponse'];

export function useDocSSE(
  docPath: string,
  enabled: boolean = true,
  workspace?: string
): SSEState<DocResponse> {
  const encodedPath = docPath.split('/').map(encodeURIComponent).join('/');
  const params = new URLSearchParams();
  if (workspace) {
    params.set('workspace', workspace);
  }
  const query = params.toString();
  const endpoint = `/events/docs/${encodedPath}${query ? `?${query}` : ''}`;
  return useSSE<DocResponse>(endpoint, enabled && !!docPath);
}
