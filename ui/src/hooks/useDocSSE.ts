import { components } from '../api/v1/schema';
import { SSEState, useSSE } from './useSSE';

type DocResponse = components['schemas']['DocResponse'];

export function useDocSSE(
  docPath: string,
  enabled: boolean = true
): SSEState<DocResponse> {
  const encodedPath = docPath
    .split('/')
    .map(encodeURIComponent)
    .join('/');
  const endpoint = `/events/docs/${encodedPath}`;
  return useSSE<DocResponse>(endpoint, enabled && !!docPath);
}
