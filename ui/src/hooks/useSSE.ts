import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';

interface SSEState<T> {
  data: T | null;
  error: Error | null;
  isConnected: boolean;
  isConnecting: boolean;
  shouldUseFallback: boolean;
}

const MAX_RETRIES = 5;
const MAX_RETRY_DELAY_MS = 16000;

function calculateRetryDelay(retryCount: number): number {
  return Math.min(1000 * Math.pow(2, retryCount), MAX_RETRY_DELAY_MS);
}

export function useSSE<T>(
  endpoint: string,
  enabled: boolean = true
): SSEState<T> {
  const appBarContext = useContext(AppBarContext);
  const config = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [state, setState] = useState<SSEState<T>>({
    data: null,
    error: null,
    isConnected: false,
    isConnecting: false,
    shouldUseFallback: false,
  });

  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const retryTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (!enabled) return;

    const url = new URL(`${config.apiURL}${endpoint}`, window.location.origin);
    url.searchParams.set('remoteNode', remoteNode);

    const token = localStorage.getItem('dagu_auth_token');
    if (token) {
      url.searchParams.set('token', token);
    }

    setState((prev) => ({ ...prev, isConnecting: true }));

    const eventSource = new EventSource(url.toString());
    eventSourceRef.current = eventSource;

    eventSource.addEventListener('connected', () => {
      setState((prev) => ({
        ...prev,
        isConnected: true,
        isConnecting: false,
        error: null,
      }));
      retryCountRef.current = 0;
    });

    eventSource.addEventListener('data', (event) => {
      try {
        const parsed = JSON.parse((event as MessageEvent).data) as T;
        setState((prev) => ({ ...prev, data: parsed }));
      } catch (e) {
        console.error('Failed to parse SSE data:', e);
      }
    });

    eventSource.addEventListener('heartbeat', () => {
      // Connection keepalive
    });

    eventSource.addEventListener('error', (event) => {
      const messageEvent = event as MessageEvent;
      if (messageEvent.data) {
        console.error('SSE error event:', messageEvent.data);
      }
    });

    eventSource.onerror = () => {
      eventSource.close();
      setState((prev) => ({
        ...prev,
        isConnected: false,
        isConnecting: false,
      }));

      if (retryCountRef.current < MAX_RETRIES) {
        const delay = calculateRetryDelay(retryCountRef.current);
        retryTimeoutRef.current = setTimeout(() => {
          retryCountRef.current++;
          connect();
        }, delay);
      } else {
        setState((prev) => ({
          ...prev,
          shouldUseFallback: true,
          error: new Error('SSE connection failed, falling back to polling'),
        }));
      }
    };
  }, [endpoint, enabled, remoteNode, config.apiURL]);

  useEffect(() => {
    connect();
    return () => {
      if (retryTimeoutRef.current !== null) {
        clearTimeout(retryTimeoutRef.current);
      }
      eventSourceRef.current?.close();
    };
  }, [connect]);

  return state;
}
