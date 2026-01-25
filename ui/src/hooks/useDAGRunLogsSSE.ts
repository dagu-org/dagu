import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

type NodeStatus = components['schemas']['NodeStatus'];

interface SchedulerLogInfo {
  content: string;
  lineCount: number;
  totalLines: number;
  hasMore: boolean;
}

interface StepLogInfo {
  stepName: string;
  status: NodeStatus;
  statusLabel: string;
  startedAt: string;
  finishedAt: string;
  hasStdout: boolean;
  hasStderr: boolean;
}

interface DAGRunLogsSSEResponse {
  schedulerLog: SchedulerLogInfo;
  stepLogs: StepLogInfo[];
}

export function useDAGRunLogsSSE(
  name: string,
  dagRunId: string,
  enabled: boolean = true
) {
  const endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}/logs`;
  return useSSE<DAGRunLogsSSEResponse>(endpoint, enabled);
}

export type { SchedulerLogInfo, StepLogInfo, DAGRunLogsSSEResponse };
