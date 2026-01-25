import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';

export interface SSEState<T> {
  data: T | null;
  error: Error | null;
  isConnected: boolean;
  isConnecting: boolean;
  shouldUseFallback: boolean;
}

const MAX_RETRIES = 5;
const MAX_RETRY_DELAY_MS = 16000;
const INITIAL_STATE: SSEState<unknown> = {
  data: null,
  error: null,
  isConnected: false,
  isConnecting: false,
  shouldUseFallback: false,
};

function calculateRetryDelay(retryCount: number): number {
  return Math.min(1000 * 2 ** retryCount, MAX_RETRY_DELAY_MS);
}

function buildSSEUrl(apiURL: string, endpoint: string, remoteNode: string): URL {
  const url = new URL(`${apiURL}${endpoint}`, window.location.origin);
  url.searchParams.set('remoteNode', remoteNode);

  const token = localStorage.getItem('dagu_auth_token');
  if (token) {
    url.searchParams.set('token', token);
  }

  return url;
}

export function useSSE<T>(endpoint: string, enabled: boolean = true): SSEState<T> {
  const appBarContext = useContext(AppBarContext);
  const config = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [state, setState] = useState<SSEState<T>>(INITIAL_STATE as SSEState<T>);

  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const retryTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (!enabled) return;

    const url = buildSSEUrl(config.apiURL, endpoint, remoteNode);

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
      const messageEvent = event as MessageEvent;
      try {
        const parsed = JSON.parse(messageEvent.data) as T;
        setState((prev) => ({ ...prev, data: parsed }));
      } catch (err) {
        console.error('SSE JSON parse error:', err);
      }
    });

    eventSource.addEventListener('heartbeat', () => {});

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
        retryCountRef.current++;
        const delay = calculateRetryDelay(retryCountRef.current - 1);
        retryTimeoutRef.current = setTimeout(() => {
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
    // Reset state and retry counter when dependencies change
    setState(INITIAL_STATE as SSEState<T>);
    retryCountRef.current = 0;

    connect();
    return () => {
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
      }
      eventSourceRef.current?.close();
    };
  }, [connect]);

  return state;
}
