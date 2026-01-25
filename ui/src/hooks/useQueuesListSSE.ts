import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

type Queue = components['schemas']['Queue'];
type QueuesSummary = components['schemas']['QueuesSummary'];

interface QueuesListSSEResponse {
  queues: Queue[];
  summary: QueuesSummary;
}

export function useQueuesListSSE(enabled: boolean = true): SSEState<QueuesListSSEResponse> {
  return useSSE<QueuesListSSEResponse>('/events/queues', enabled);
}

export type { QueuesListSSEResponse };
