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

const MAX_RETRY_DELAY_MS = 16000;

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

    let disposed = false;
    let eventSource: EventSource | null = null;
    let retryTimeout: ReturnType<typeof setTimeout> | null = null;
    let retryCount = 0;

    function connect() {
      if (disposed) return;

      eventSource = new EventSource(
        buildAgentStreamUrl(apiURL, remoteNode, sessionId!)
      );

      eventSource.onopen = () => {
        if (disposed) return;
        retryCount = 0;
        awaitingSnapshotRef.current = true;
        setIsSessionLive(true);
      };

      eventSource.addEventListener('message', (event) => {
        if (disposed) return;

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
        if (disposed) return;
        // Close immediately to free the HTTP connection slot. Without this,
        // the browser's built-in EventSource reconnect holds the slot while
        // retrying, which can exhaust the 6-connection-per-origin budget.
        eventSource?.close();
        eventSource = null;
        setIsSessionLive(false);

        // Reconnect with exponential backoff.
        const delay = Math.min(1000 * 2 ** retryCount, MAX_RETRY_DELAY_MS);
        retryCount++;
        retryTimeout = setTimeout(connect, delay);
      };
    }

    connect();

    return () => {
      disposed = true;
      if (retryTimeout) clearTimeout(retryTimeout);
      if (eventSource) eventSource.close();
      setIsSessionLive(false);
    };
  }, [sessionId, apiURL, remoteNode]);

  return { isSessionLive };
}
