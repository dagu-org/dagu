import { components } from '../api/v2/schema';
import { useSSE } from './useSSE';

type DAGDetails = components['schemas']['DAGDetails'];
type DAGRunDetails = components['schemas']['DAGRunDetails'];
type LocalDag = components['schemas']['LocalDag'];

interface DAGSSEResponse {
  dag: DAGDetails;
  latestDAGRun: DAGRunDetails;
  suspended: boolean;
  localDags: LocalDag[];
  errors: string[];
}

export function useDAGSSE(fileName: string, enabled: boolean = true) {
  const endpoint = `/events/dags/${encodeURIComponent(fileName)}`;
  return useSSE<DAGSSEResponse>(endpoint, enabled);
}
