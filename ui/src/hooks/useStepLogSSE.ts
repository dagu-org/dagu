import { useSSE } from './useSSE';

interface StepLogSSEResponse {
  stdoutContent: string;
  stderrContent: string;
  lineCount: number;
  totalLines: number;
  hasMore: boolean;
}

export function useStepLogSSE(
  name: string,
  dagRunId: string,
  stepName: string,
  enabled: boolean = true
) {
  const endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}/logs/steps/${encodeURIComponent(stepName)}`;
  return useSSE<StepLogSSEResponse>(endpoint, enabled);
}

export type { StepLogSSEResponse };
