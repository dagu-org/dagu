// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { getAuthToken } from '@/lib/authHeaders';

const MAX_RETRY_DELAY_MS = 16000;
const CONNECT_TIMEOUT_MS = 15000;
const DRAINING_GRACE_PERIOD_MS = 2000;

export type AppLiveEvent =
  | {
      type: 'connected';
      node?: string;
      serverTime?: string;
      version?: number;
    }
  | {
      type: 'reset';
      reason?: string;
    }
  | {
      type: 'dag.changed';
      fileName?: string;
      reason?: string;
    }
  | {
      type: 'dagrun.changed';
      reason?: string;
    }
  | {
      type: 'queue.changed';
      queueName?: string;
      reason?: string;
    }
  | {
      type: 'doc.changed';
      path?: string;
      reason?: string;
    };

export interface LiveConnectionState {
  isConnected: boolean;
  isConnecting: boolean;
  shouldUseFallback: boolean;
  error: Error | null;
}

interface SubscriberCallbacks {
  matches: (event: AppLiveEvent) => boolean;
  onInvalidate: (event: AppLiveEvent) => void;
  onStateChange: (state: LiveConnectionState) => void;
}

interface ManagedConnection {
  key: string;
  apiURL: string;
  remoteNode: string;
  eventSource: EventSource | null;
  retryCount: number;
  retryTimeout: ReturnType<typeof setTimeout> | null;
  connectTimeout: ReturnType<typeof setTimeout> | null;
  drainTimeout: ReturnType<typeof setTimeout> | null;
  wasEverConnected: boolean;
  state: LiveConnectionState;
  subscribers: Map<string, SubscriberCallbacks>;
}

const INITIAL_STATE: LiveConnectionState = {
  isConnected: false,
  isConnecting: false,
  shouldUseFallback: true,
  error: null,
};

let subscriberIdCounter = 0;

function buildConnectionKey(apiURL: string, remoteNode: string): string {
  return `${apiURL}|${remoteNode}`;
}

function buildStreamUrl(apiURL: string, remoteNode: string): string {
  const url = new URL(`${apiURL}/events/app`, window.location.origin);
  url.searchParams.set('remoteNode', remoteNode);

  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }

  return url.toString();
}

function calculateRetryDelay(retryCount: number): number {
  return Math.min(1000 * 2 ** retryCount, MAX_RETRY_DELAY_MS);
}

const EVENT_TYPES: AppLiveEvent['type'][] = [
  'connected',
  'reset',
  'dag.changed',
  'dagrun.changed',
  'queue.changed',
  'doc.changed',
];

export class AppLiveManager {
  private connections = new Map<string, ManagedConnection>();

  subscribe(
    remoteNode: string,
    apiURL: string,
    callbacks: SubscriberCallbacks
  ): () => void {
    const key = buildConnectionKey(apiURL, remoteNode);
    const subscriberId = String(++subscriberIdCounter);
    const conn = this.getOrCreateConnection(key, apiURL, remoteNode);

    if (conn.drainTimeout) {
      clearTimeout(conn.drainTimeout);
      conn.drainTimeout = null;
    }

    conn.subscribers.set(subscriberId, callbacks);
    callbacks.onStateChange(conn.state);
    this.ensureConnected(conn);

    return () => this.unsubscribe(key, subscriberId);
  }

  private getOrCreateConnection(
    key: string,
    apiURL: string,
    remoteNode: string
  ): ManagedConnection {
    const existing = this.connections.get(key);
    if (existing) {
      return existing;
    }

    const conn: ManagedConnection = {
      key,
      apiURL,
      remoteNode,
      eventSource: null,
      retryCount: 0,
      retryTimeout: null,
      connectTimeout: null,
      drainTimeout: null,
      wasEverConnected: false,
      state: INITIAL_STATE,
      subscribers: new Map(),
    };
    this.connections.set(key, conn);
    return conn;
  }

  private unsubscribe(key: string, subscriberId: string): void {
    const conn = this.connections.get(key);
    if (!conn) {
      return;
    }

    conn.subscribers.delete(subscriberId);
    if (conn.subscribers.size > 0) {
      return;
    }

    conn.drainTimeout = setTimeout(() => {
      if (conn.subscribers.size > 0) {
        return;
      }
      this.disposeConnection(conn);
    }, DRAINING_GRACE_PERIOD_MS);
  }

  private ensureConnected(conn: ManagedConnection): void {
    if (conn.subscribers.size === 0) {
      return;
    }
    if (conn.eventSource || conn.retryTimeout) {
      return;
    }
    this.connect(conn);
  }

  private connect(conn: ManagedConnection): void {
    if (conn.subscribers.size === 0) {
      return;
    }

    if (conn.eventSource) {
      conn.eventSource.close();
      conn.eventSource = null;
    }
    if (conn.connectTimeout) {
      clearTimeout(conn.connectTimeout);
      conn.connectTimeout = null;
    }

    const url = buildStreamUrl(conn.apiURL, conn.remoteNode);
    const eventSource = new EventSource(url);
    const shouldRefreshOnOpen = conn.wasEverConnected || conn.retryCount > 0;

    conn.eventSource = eventSource;
    this.updateState(conn, {
      isConnected: false,
      isConnecting: true,
      shouldUseFallback: true,
      error: null,
    });

    conn.connectTimeout = setTimeout(() => {
      if (conn.eventSource !== eventSource || conn.state.isConnected) {
        return;
      }
      eventSource.close();
      conn.eventSource = null;
      this.handleDisconnect(conn, new Error('App live connect timeout'));
    }, CONNECT_TIMEOUT_MS);

    eventSource.addEventListener('open', () => {
      if (conn.eventSource !== eventSource) {
        return;
      }
      conn.retryCount = 0;
      conn.wasEverConnected = true;
      if (conn.connectTimeout) {
        clearTimeout(conn.connectTimeout);
        conn.connectTimeout = null;
      }
      this.updateState(conn, {
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
        error: null,
      });
      if (shouldRefreshOnOpen) {
        this.notifySubscribers(conn, { type: 'reset', reason: 'reconnected' });
      }
    });

    for (const eventType of EVENT_TYPES) {
      eventSource.addEventListener(eventType, (event) => {
        if (conn.eventSource !== eventSource) {
          return;
        }
        try {
          const parsed = JSON.parse((event as MessageEvent).data) as AppLiveEvent;
          this.notifySubscribers(conn, parsed);
        } catch (error) {
          this.updateState(conn, {
            error:
              error instanceof Error
                ? error
                : new Error('Invalid JSON response from app live stream'),
          });
        }
      });
    }

    eventSource.onerror = () => {
      if (conn.eventSource !== eventSource) {
        return;
      }
      eventSource.close();
      conn.eventSource = null;
      this.handleDisconnect(conn, new Error('App live connection lost'));
    };
  }

  private handleDisconnect(conn: ManagedConnection, error: Error): void {
    if (conn.connectTimeout) {
      clearTimeout(conn.connectTimeout);
      conn.connectTimeout = null;
    }

    this.updateState(conn, {
      isConnected: false,
      isConnecting: false,
      shouldUseFallback: true,
      error,
    });

    if (conn.subscribers.size === 0) {
      return;
    }

    const delay = calculateRetryDelay(conn.retryCount);
    conn.retryCount += 1;
    conn.retryTimeout = setTimeout(() => {
      conn.retryTimeout = null;
      this.connect(conn);
    }, delay);
  }

  private notifySubscribers(conn: ManagedConnection, event: AppLiveEvent): void {
    for (const subscriber of conn.subscribers.values()) {
      try {
        if (event.type === 'reset' || subscriber.matches(event)) {
          subscriber.onInvalidate(event);
        }
      } catch (error) {
        console.error('AppLiveManager subscriber invalidate failed', error);
      }
    }
  }

  private updateState(
    conn: ManagedConnection,
    partial: Partial<LiveConnectionState>
  ): void {
    conn.state = {
      ...conn.state,
      ...partial,
    };

    for (const subscriber of conn.subscribers.values()) {
      try {
        subscriber.onStateChange(conn.state);
      } catch (error) {
        console.error('AppLiveManager subscriber state change failed', error);
      }
    }
  }

  private disposeConnection(conn: ManagedConnection): void {
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
    if (conn.drainTimeout) {
      clearTimeout(conn.drainTimeout);
      conn.drainTimeout = null;
    }

    this.connections.delete(conn.key);
  }
}

export const appLiveManager = new AppLiveManager();
