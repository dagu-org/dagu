import { components } from '../api/v1/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface QueueItemsSSEResponse {
  running: DAGRunSummary[];
  queued: DAGRunSummary[];
}

export function useQueueItemsSSE(
  queueName: string,
  enabled: boolean = true
): SSEState<QueueItemsSSEResponse> {
  const endpoint = `/events/queues/${encodeURIComponent(queueName)}/items`;
  return useSSE<QueueItemsSSEResponse>(endpoint, enabled);
}

export type { QueueItemsSSEResponse };
