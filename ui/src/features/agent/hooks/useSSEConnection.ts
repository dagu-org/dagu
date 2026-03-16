import { useEffect, useRef, useState } from 'react';
import { getAuthToken } from '@/lib/authHeaders';
import { StreamResponse } from '../types';

export interface SSECallbacks {
  onEvent: (event: StreamResponse, replace: boolean) => void;
  onNavigate: (path: string) => void;
}

export interface AgentSSEStatus {
  isSessionLive: boolean;
}

function buildAgentStreamUrl(
  apiURL: string,
  remoteNode: string,
  sessionId: string
): string {
  const url = new URL(
    `${apiURL}/agent/sessions/${encodeURIComponent(sessionId)}/stream`,
    window.location.origin
  );
  url.searchParams.set('remoteNode', remoteNode);

  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }

  return url.toString();
}

export function useSSEConnection(
  sessionId: string | null,
  apiURL: string,
  remoteNode: string,
  callbacks: SSECallbacks
): AgentSSEStatus {
  const handledNavigateIdsRef = useRef<Set<string>>(new Set());
  const awaitingSnapshotRef = useRef(false);
  const connectionGenerationRef = useRef(0);
  const cbRef = useRef(callbacks);
  cbRef.current = callbacks;

  const [isSessionLive, setIsSessionLive] = useState(false);

  useEffect(() => {
    handledNavigateIdsRef.current = new Set();
    awaitingSnapshotRef.current = false;
    setIsSessionLive(false);
  }, [sessionId]);

  useEffect(() => {
    if (!sessionId) {
      setIsSessionLive(false);
      return;
    }

    const generation = connectionGenerationRef.current + 1;
    connectionGenerationRef.current = generation;
    let closed = false;
    const eventSource = new EventSource(
      buildAgentStreamUrl(apiURL, remoteNode, sessionId)
    );

    const isCurrentConnection = () =>
      !closed && connectionGenerationRef.current === generation;

    eventSource.onopen = () => {
      if (!isCurrentConnection()) {
        return;
      }
      awaitingSnapshotRef.current = true;
      setIsSessionLive(true);
    };

    eventSource.addEventListener('message', (event) => {
      if (!isCurrentConnection()) {
        return;
      }

      try {
        const parsed = JSON.parse(
          (event as MessageEvent<string>).data
        ) as StreamResponse;
        const replace = awaitingSnapshotRef.current;
        awaitingSnapshotRef.current = false;
        cbRef.current.onEvent(parsed, replace);

        for (const msg of parsed.messages ?? []) {
          if (
            msg.id &&
            msg.type === 'ui_action' &&
            msg.ui_action?.type === 'navigate' &&
            msg.ui_action.path &&
            !handledNavigateIdsRef.current.has(msg.id)
          ) {
            handledNavigateIdsRef.current.add(msg.id);
            cbRef.current.onNavigate(msg.ui_action.path);
          }
        }
      } catch (error) {
        console.error('Invalid JSON response from agent SSE', error);
      }
    });

    eventSource.onerror = () => {
      if (!isCurrentConnection()) {
        return;
      }
      // Keep the EventSource alive so the browser can apply its built-in
      // reconnect policy while the polling fallback covers the gap.
      setIsSessionLive(false);
    };

    return () => {
      closed = true;
      eventSource.close();
      if (connectionGenerationRef.current === generation) {
        setIsSessionLive(false);
      }
    };
  }, [sessionId, apiURL, remoteNode]);

  return { isSessionLive };
}
