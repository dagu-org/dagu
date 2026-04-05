// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { components } from '../api/v1/schema';
import { SSEState, useSSE } from './useSSE';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

interface DAGRunSSEResponse {
  dagRunDetails: DAGRunDetails;
}

export function useDAGRunSSE(
  name: string,
  dagRunId: string,
  enabled: boolean = true,
  remoteNode?: string
): SSEState<DAGRunSSEResponse> {
  const endpoint = `/events/dag-runs/${encodeURIComponent(name)}/${encodeURIComponent(dagRunId)}`;
  return useSSE<DAGRunSSEResponse>(endpoint, enabled, remoteNode);
}

export type { DAGRunSSEResponse };
