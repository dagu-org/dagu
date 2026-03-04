/**
 * Centralized SSE connection manager.
 *
 * Solves browser HTTP/1.1 connection exhaustion (Chrome's 6-connection-per-origin
 * limit) by managing a pool of EventSource instances with:
 * - Ref-counted deduplication: same endpoint → shared EventSource
 * - Connection cap (default: 3) with LRU eviction to SWR polling fallback
 * - 15s connect timeout: releases browser slots for queued connections
 * - Per-connection cleanup debounce: handles React StrictMode double-mount
 *
 * @module hooks/SSEManager
 */

const MAX_CONNECTIONS = 3;
const MAX_RETRIES = 5;
const MAX_RETRY_DELAY_MS = 16000;
const CONNECT_TIMEOUT_MS = 15000;
const CLEANUP_DEBOUNCE_MS = 100;

export interface SSEConnectionState {
  isConnected: boolean;
  isConnecting: boolean;
  shouldUseFallback: boolean;
  error: Error | null;
}

export interface SubscriberCallbacks {
  onData: (data: unknown) => void;
  onStateChange: (state: SSEConnectionState) => void;
}

interface ManagedConnection {
  key: string;
  apiURL: string;
  endpoint: string;
  remoteNode: string;
  eventSource: EventSource | null;
  subscribers: Map<string, SubscriberCallbacks>;
  lastActivity: number;
  retryCount: number;
  retryTimeout: ReturnType<typeof setTimeout> | null;
  connectTimeout: ReturnType<typeof setTimeout> | null;
  cleanupTimeout: ReturnType<typeof setTimeout> | null;
  state: SSEConnectionState;
}

let subscriberIdCounter = 0;

function buildConnectionKey(apiURL: string, remoteNode: string, endpoint: string): string {
  return `${apiURL}|${remoteNode}|${endpoint}`;
}

function buildSSEUrl(apiURL: string, endpoint: string, remoteNode: string): string {
  const url = new URL(`${apiURL}${endpoint}`, window.location.origin);
  url.searchParams.set('remoteNode', remoteNode);

  const token = localStorage.getItem('dagu_auth_token');
  if (token) {
    url.searchParams.set('token', token);
  }

  return url.toString();
}

function calculateRetryDelay(retryCount: number): number {
  return Math.min(1000 * 2 ** retryCount, MAX_RETRY_DELAY_MS);
}

class SSEManager {
  private connections = new Map<string, ManagedConnection>();
  private maxConnections = MAX_CONNECTIONS;

  subscribe(
    endpoint: string,
    remoteNode: string,
    apiURL: string,
    callbacks: SubscriberCallbacks
  ): () => void {
    const key = buildConnectionKey(apiURL, remoteNode, endpoint);
    const subscriberId = String(++subscriberIdCounter);

    let conn = this.connections.get(key);

    if (conn) {
      // Reuse existing connection — cancel any pending cleanup
      if (conn.cleanupTimeout) {
        clearTimeout(conn.cleanupTimeout);
        conn.cleanupTimeout = null;
      }
      conn.subscribers.set(subscriberId, callbacks);
      // Immediately notify subscriber of current state
      callbacks.onStateChange(conn.state);
      return () => this.unsubscribe(key, subscriberId);
    }

    // New connection needed — check capacity
    if (this.connections.size >= this.maxConnections) {
      this.evictLRU();
    }

    conn = {
      key,
      apiURL,
      endpoint,
      remoteNode,
      eventSource: null,
      subscribers: new Map([[subscriberId, callbacks]]),
      lastActivity: Date.now(),
      retryCount: 0,
      retryTimeout: null,
      connectTimeout: null,
      cleanupTimeout: null,
      state: {
        isConnected: false,
        isConnecting: true,
        shouldUseFallback: false,
        error: null,
      },
    };

    this.connections.set(key, conn);
    this.connect(conn);

    return () => this.unsubscribe(key, subscriberId);
  }

  private connect(conn: ManagedConnection): void {
    // Close existing EventSource if any (reconnection case)
    if (conn.eventSource) {
      conn.eventSource.close();
      conn.eventSource = null;
    }
    if (conn.connectTimeout) {
      clearTimeout(conn.connectTimeout);
      conn.connectTimeout = null;
    }

    const url = buildSSEUrl(conn.apiURL, conn.endpoint, conn.remoteNode);

    this.updateState(conn, { isConnecting: true, isConnected: false });

    const eventSource = new EventSource(url);
    conn.eventSource = eventSource;

    // 15s connect timeout — release browser slot if connection is queued
    conn.connectTimeout = setTimeout(() => {
      if (conn.state.isConnecting && !conn.state.isConnected) {
        eventSource.close();
        conn.eventSource = null;
        this.updateState(conn, {
          isConnecting: false,
          isConnected: false,
          shouldUseFallback: true,
          error: new Error('SSE connect timeout, falling back to polling'),
        });
      }
    }, CONNECT_TIMEOUT_MS);

    eventSource.addEventListener('connected', () => {
      // Clear connect timeout
      if (conn.connectTimeout) {
        clearTimeout(conn.connectTimeout);
        conn.connectTimeout = null;
      }
      conn.retryCount = 0;
      this.updateState(conn, {
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
        error: null,
      });
    });

    eventSource.addEventListener('data', (event) => {
      const messageEvent = event as MessageEvent;
      try {
        const parsed = JSON.parse(messageEvent.data);
        // Update LRU on data events only (not heartbeats)
        conn.lastActivity = Date.now();
        this.notifyData(conn, parsed);
      } catch (err) {
        console.error('SSE JSON parse error:', err);
        this.updateState(conn, {
          error: new Error('Invalid JSON response from SSE'),
        });
      }
    });

    // Listen for heartbeat to reset retry counter
    eventSource.addEventListener('heartbeat', () => {
      conn.retryCount = 0;
    });

    eventSource.addEventListener('error', (event) => {
      const messageEvent = event as MessageEvent;
      if (messageEvent.data) {
        console.error('SSE error event:', messageEvent.data);
      }
    });

    eventSource.onerror = () => {
      eventSource.close();
      conn.eventSource = null;

      if (conn.connectTimeout) {
        clearTimeout(conn.connectTimeout);
        conn.connectTimeout = null;
      }

      this.updateState(conn, {
        isConnected: false,
        isConnecting: false,
      });

      if (conn.retryCount < MAX_RETRIES) {
        const delay = calculateRetryDelay(conn.retryCount);
        conn.retryCount++;
        conn.retryTimeout = setTimeout(() => {
          conn.retryTimeout = null;
          // Only reconnect if still has subscribers
          if (conn.subscribers.size > 0) {
            this.connect(conn);
          }
        }, delay);
      } else {
        this.updateState(conn, {
          shouldUseFallback: true,
          error: new Error('SSE connection failed, falling back to polling'),
        });
      }
    };
  }

  private unsubscribe(key: string, subscriberId: string): void {
    const conn = this.connections.get(key);
    if (!conn) return;

    conn.subscribers.delete(subscriberId);

    if (conn.subscribers.size === 0) {
      // Debounce cleanup to handle React StrictMode double-mount
      conn.cleanupTimeout = setTimeout(() => {
        // Re-check — a new subscriber may have arrived during the debounce
        if (conn.subscribers.size === 0) {
          this.closeConnection(conn);
          this.connections.delete(key);
        }
      }, CLEANUP_DEBOUNCE_MS);
    }
  }

  private evictLRU(): void {
    let oldest: ManagedConnection | null = null;
    for (const conn of this.connections.values()) {
      if (!oldest || conn.lastActivity < oldest.lastActivity) {
        oldest = conn;
      }
    }

    if (oldest) {
      this.closeConnection(oldest);
      // Notify evicted subscribers to fall back to polling
      this.updateState(oldest, {
        isConnected: false,
        isConnecting: false,
        shouldUseFallback: true,
        error: null,
      });
      this.connections.delete(oldest.key);
    }
  }

  private closeConnection(conn: ManagedConnection): void {
    if (conn.eventSource) {
      conn.eventSource.close();
      conn.eventSource = null;
    }
    if (conn.retryTimeout) {
      clearTimeout(conn.retryTimeout);
      conn.retryTimeout = null;
    }
    if (conn.connectTimeout) {
      clearTimeout(conn.connectTimeout);
      conn.connectTimeout = null;
    }
    if (conn.cleanupTimeout) {
      clearTimeout(conn.cleanupTimeout);
      conn.cleanupTimeout = null;
    }
  }

  private updateState(conn: ManagedConnection, partial: Partial<SSEConnectionState>): void {
    conn.state = { ...conn.state, ...partial };
    for (const cb of conn.subscribers.values()) {
      cb.onStateChange(conn.state);
    }
  }

  private notifyData(conn: ManagedConnection, data: unknown): void {
    for (const cb of conn.subscribers.values()) {
      cb.onData(data);
    }
  }
}

export const sseManager = new SSEManager();
