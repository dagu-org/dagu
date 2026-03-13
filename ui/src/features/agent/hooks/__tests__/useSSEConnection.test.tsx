import { renderHook, act } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  SubscriberCallbacks,
  SSEConnectionState,
} from '@/hooks/SSEManager';
import { useSSEConnection } from '../useSSEConnection';

const subscribeTopicMock = vi.fn();
const topicSubscribers = new Map<string, SubscriberCallbacks>();

vi.mock('@/hooks/SSEManager', () => ({
  buildAgentSessionTopic: (sessionId: string) => `agent:${sessionId}`,
  sseManager: {
    subscribeTopic: (...args: unknown[]) => subscribeTopicMock(...args),
  },
}));

function emitState(topic: string, state: SSEConnectionState) {
  const subscriber = topicSubscribers.get(topic);
  if (!subscriber) {
    throw new Error(`missing subscriber for topic ${topic}`);
  }
  subscriber.onStateChange(state);
}

function emitData(topic: string, data: unknown) {
  const subscriber = topicSubscribers.get(topic);
  if (!subscriber) {
    throw new Error(`missing subscriber for topic ${topic}`);
  }
  subscriber.onData(data);
}

describe('useSSEConnection', () => {
  beforeEach(() => {
    topicSubscribers.clear();
    subscribeTopicMock.mockReset();
    subscribeTopicMock.mockImplementation(
      (
        topic: string,
        _remoteNode: string,
        _apiURL: string,
        callbacks: SubscriberCallbacks
      ) => {
        topicSubscribers.set(topic, callbacks);
        callbacks.onStateChange({
          isConnected: false,
          isConnecting: true,
          shouldUseFallback: false,
          error: null,
        });
        return () => {
          topicSubscribers.delete(topic);
        };
      }
    );
  });

  it('tracks primary and delegate liveness from SSE manager state', () => {
    const onSnapshot = vi.fn();
    const onDelegateSnapshot = vi.fn();
    const onNavigate = vi.fn();
    const onPreConnect = vi.fn();

    const { result } = renderHook(() =>
      useSSEConnection('sess-1', ['delegate-1'], '/api/v1', 'local', {
        onSnapshot,
        onDelegateSnapshot,
        onNavigate,
        onPreConnect,
      })
    );

    expect(onPreConnect).toHaveBeenCalledTimes(1);
    expect(result.current.isSessionLive).toBe(false);
    expect(result.current.liveDelegateSessions['delegate-1']).toBe(false);

    act(() => {
      emitState('agent:sess-1', {
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
        error: null,
      });
      emitState('agent:delegate-1', {
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
        error: null,
      });
    });

    expect(result.current.isSessionLive).toBe(true);
    expect(result.current.liveDelegateSessions['delegate-1']).toBe(true);

    act(() => {
      emitState('agent:delegate-1', {
        isConnected: false,
        isConnecting: false,
        shouldUseFallback: true,
        error: new Error('down'),
      });
    });

    expect(result.current.liveDelegateSessions['delegate-1']).toBe(false);
  });

  it('deduplicates repeated navigate UI actions while still forwarding snapshots', () => {
    const onSnapshot = vi.fn();
    const onDelegateSnapshot = vi.fn();
    const onNavigate = vi.fn();
    const onPreConnect = vi.fn();

    renderHook(() =>
      useSSEConnection('sess-1', [], '/api/v1', 'local', {
        onSnapshot,
        onDelegateSnapshot,
        onNavigate,
        onPreConnect,
      })
    );

    const snapshot = {
      messages: [
        {
          id: 'msg-1',
          type: 'ui_action',
          ui_action: { type: 'navigate', path: '/runs/run-1' },
        },
      ],
    };

    act(() => {
      emitData('agent:sess-1', snapshot);
      emitData('agent:sess-1', snapshot);
    });

    expect(onSnapshot).toHaveBeenCalledTimes(2);
    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(onNavigate).toHaveBeenCalledWith('/runs/run-1');
    expect(onDelegateSnapshot).not.toHaveBeenCalled();
  });
});
