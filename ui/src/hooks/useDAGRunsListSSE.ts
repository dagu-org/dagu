// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { components } from '../api/v1/schema';
import { buildSSEEndpoint, SSEState, useSSE } from './useSSE';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface DAGRunsListSSEResponse {
  dagRuns: DAGRunSummary[];
}

interface DAGRunsListParams {
  status?: number;
  fromDate?: number;
  toDate?: number;
  name?: string;
  dagRunId?: string;
  tags?: string;
}

export function useDAGRunsListSSE(
  params: DAGRunsListParams = {},
  enabled: boolean = true,
  remoteNode?: string
): SSEState<DAGRunsListSSEResponse> {
  const endpoint = buildSSEEndpoint('/events/dag-runs', params);
  return useSSE<DAGRunsListSSEResponse>(endpoint, enabled, remoteNode);
}

export type { DAGRunsListParams, DAGRunsListSSEResponse };
