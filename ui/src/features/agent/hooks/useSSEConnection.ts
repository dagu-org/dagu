import { StreamResponse } from '../types';

export interface SSECallbacks {
  onEvent: (event: StreamResponse, replace: boolean) => void;
  onNavigate: (path: string) => void;
}

export interface AgentSSEStatus {
  isSessionLive: boolean;
}

/**
 * Agent SSE is intentionally disabled. The polling fallback in useAgentChat
 * handles all session updates via periodic GET requests.
 *
 * Why: Each EventSource holds a permanent HTTP/1.1 connection slot (browsers
 * allow only 6 per origin). The multiplexed SSE for dashboard/cockpit already
 * uses one slot. Adding a second for the agent leaves only 4 slots for all
 * fetch requests, which causes deadlocks when any request is slow (AI
 * generation, server lag). Removing the agent EventSource frees a slot and
 * eliminates the entire class of connection-starvation bugs.
 *
 * The 2s polling interval is imperceptible in a chat UI where the user is
 * already waiting for AI responses.
 */
export function useSSEConnection(
  _sessionId: string | null,
  _apiURL: string,
  _remoteNode: string,
  _callbacks: SSECallbacks
): AgentSSEStatus {
  void _sessionId;
  void _apiURL;
  void _remoteNode;
  void _callbacks;
  return { isSessionLive: false };
}
