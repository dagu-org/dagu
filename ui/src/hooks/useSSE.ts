import { useCallback, useEffect, useRef, useState, useContext } from 'react';
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

    // Get auth token
    const token = localStorage.getItem('dagu_auth_token');

    // Build SSE URL
    const url = new URL(`${config.apiURL}${endpoint}`, window.location.origin);
    url.searchParams.set('remoteNode', remoteNode);
    if (token) {
      url.searchParams.set('token', token);
    }

    setState((prev) => ({ ...prev, isConnecting: true }));

    const es = new EventSource(url.toString());
    eventSourceRef.current = es;

    es.addEventListener('connected', () => {
      setState((prev) => ({
        ...prev,
        isConnected: true,
        isConnecting: false,
        error: null,
      }));
      retryCountRef.current = 0;
    });

    es.addEventListener('data', (event) => {
      try {
        const parsed = JSON.parse((event as MessageEvent).data) as T;
        setState((prev) => ({ ...prev, data: parsed }));
      } catch (e) {
        console.error('Failed to parse SSE data:', e);
      }
    });

    es.addEventListener('heartbeat', () => {
      // Heartbeat received - connection is alive
    });

    es.addEventListener('error', (event) => {
      // Check for specific error event data
      const messageEvent = event as MessageEvent;
      if (messageEvent.data) {
        console.error('SSE error event:', messageEvent.data);
      }
    });

    es.onerror = () => {
      es.close();
      setState((prev) => ({
        ...prev,
        isConnected: false,
        isConnecting: false,
      }));

      if (retryCountRef.current < MAX_RETRIES) {
        // Exponential backoff: 1s, 2s, 4s, 8s, 16s
        const delay = Math.min(
          1000 * Math.pow(2, retryCountRef.current),
          16000
        );
        retryTimeoutRef.current = setTimeout(() => {
          retryCountRef.current++;
          connect();
        }, delay);
      } else {
        // Max retries reached - fall back to polling
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
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, [connect]);

  return state;
}
