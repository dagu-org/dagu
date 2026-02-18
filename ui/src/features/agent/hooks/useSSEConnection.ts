import { useCallback, useEffect, useRef, useState } from 'react';
import { getAuthToken } from '@/lib/authHeaders';
import { MAX_SSE_RETRIES } from '../constants';
import { DelegateEvent, DelegateMessages, DelegateSnapshot, Message, StreamResponse } from '../types';

function buildStreamUrl(baseUrl: string, sessionId: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}/sessions/${sessionId}/stream`, window.location.origin);
  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }
  url.searchParams.set('remoteNode', remoteNode);
  return url.toString();
}

export interface SSECallbacks {
  onMessage: (msg: Message) => void;
  onSessionState: (state: NonNullable<StreamResponse['session_state']>) => void;
  onDelegateSnapshots: (snapshots: DelegateSnapshot[]) => void;
  onDelegateMessages: (dm: DelegateMessages) => void;
  onDelegateEvent: (evt: DelegateEvent) => void;
  onNavigate: (path: string) => void;
  onPreConnect: () => void;
}

export function useSSEConnection(
  sessionId: string | null,
  baseUrl: string,
  remoteNode: string,
  callbacks: SSECallbacks
) {
  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const [sseRetryTrigger, setSseRetryTrigger] = useState(0);

  const closeEventSource = useCallback((): void => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  useEffect(() => {
    return closeEventSource;
  }, [closeEventSource]);

  // Stable ref for callbacks so the SSE effect doesn't re-fire on every render.
  const cbRef = useRef(callbacks);
  cbRef.current = callbacks;

  useEffect(() => {
    if (!sessionId) {
      closeEventSource();
      return;
    }

    eventSourceRef.current?.close();

    cbRef.current.onPreConnect();

    const eventSource = new EventSource(buildStreamUrl(baseUrl, sessionId, remoteNode));
    eventSourceRef.current = eventSource;

    eventSource.onmessage = (event) => {
      try {
        const data: StreamResponse = JSON.parse(event.data);
        retryCountRef.current = 0;

        for (const msg of data.messages ?? []) {
          cbRef.current.onMessage(msg);
          if (msg.type === 'ui_action' && msg.ui_action?.type === 'navigate' && msg.ui_action.path) {
            cbRef.current.onNavigate(msg.ui_action.path);
          }
        }

        if (data.session_state) {
          cbRef.current.onSessionState(data.session_state);
        }

        if (data.delegates && data.delegates.length > 0) {
          cbRef.current.onDelegateSnapshots(data.delegates);
        }

        if (data.delegate_messages) {
          cbRef.current.onDelegateMessages(data.delegate_messages);
        }

        if (data.delegate_event) {
          cbRef.current.onDelegateEvent(data.delegate_event);
        }
      } catch {
        // SSE parse errors are transient, stream will continue
      }
    };

    eventSource.onerror = () => {
      if (eventSource.readyState === EventSource.CLOSED && retryCountRef.current < MAX_SSE_RETRIES) {
        const delay = 1000 * Math.pow(2, retryCountRef.current);
        retryCountRef.current++;
        setTimeout(() => {
          if (sessionId && eventSourceRef.current === eventSource) {
            setSseRetryTrigger((prev) => prev + 1);
          }
        }, delay);
      }
    };

    return () => {
      eventSource.close();
    };
  }, [sessionId, baseUrl, remoteNode, sseRetryTrigger, closeEventSource]);

  return { closeEventSource };
}
