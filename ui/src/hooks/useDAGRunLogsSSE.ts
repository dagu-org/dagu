import { components } from '../api/v2/schema';
import { SSEState, useSSE } from './useSSE';

type NodeStatus = components['schemas']['NodeStatus'];

export interface SchedulerLogInfo {
  content: string;
  lineCount: number;
  totalLines: number;
  hasMore: boolean;
}

export interface StepLogInfo {
  stepName: string;
  status: NodeStatus;
  statusLabel: string;
  startedAt: string;
  finishedAt: string;
  hasStdout: boolean;
  hasStderr: boolean;
}

export interface DAGRunLogsSSEResponse {
  schedulerLog: SchedulerLogInfo;
  stepLogs: StepLogInfo[];
}

export function useDAGRunLogsSSE(
  name: string,
  dagRunId: string,
  enabled: boolean = true,
  tail?: number
): SSEState<DAGRunLogsSSEResponse> {
  let endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}/logs`;
  if (tail !== undefined) {
    endpoint += `?tail=${tail}`;
  }
  return useSSE<DAGRunLogsSSEResponse>(endpoint, enabled);
}
