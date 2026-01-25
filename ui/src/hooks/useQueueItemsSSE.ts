import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface QueueItemsSSEResponse {
  running: DAGRunSummary[];
  queued: DAGRunSummary[];
}

export function useQueueItemsSSE(queueName: string, enabled: boolean = true) {
  const endpoint = `/events/queues/${encodeURIComponent(queueName)}/items`;
  return useSSE<QueueItemsSSEResponse>(endpoint, enabled);
}

export type { QueueItemsSSEResponse };
