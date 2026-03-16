import { renderHook, act } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSSEConnection } from '../useSSEConnection';

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  private listeners = new Map<
    string,
    Set<(event: MessageEvent<string>) => void>
  >();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  addEventListener(
    type: string,
    listener: (event: MessageEvent<string>) => void
  ) {
    const listeners = this.listeners.get(type) ?? new Set();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }

  open() {
    this.onopen?.();
  }

  error() {
    this.onerror?.();
  }

  emitMessage(data: unknown) {
    const listeners = this.listeners.get('message');
    if (!listeners) {
      return;
    }

    const event = {
      data: JSON.stringify(data),
    } as MessageEvent<string>;
    for (const listener of listeners) {
      listener(event);
    }
  }
}

describe('useSSEConnection', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    vi.stubGlobal('EventSource', MockEventSource);
  });

  it('marks the session live on open and treats the first event after each open as a snapshot replace', () => {
    const onEvent = vi.fn();
    const onNavigate = vi.fn();

    const { result } = renderHook(() =>
      useSSEConnection('sess-1', '/api/v1', 'local', {
        onEvent,
        onNavigate,
      })
    );

    expect(result.current.isSessionLive).toBe(false);
    expect(MockEventSource.instances).toHaveLength(1);

    const eventSource = MockEventSource.instances[0];
    if (!eventSource) {
      throw new Error('expected EventSource instance');
    }

    act(() => {
      eventSource.open();
    });

    expect(result.current.isSessionLive).toBe(true);

    act(() => {
      eventSource.emitMessage({ messages: [{ id: 'm1', type: 'assistant' }] });
      eventSource.emitMessage({ messages: [{ id: 'm2', type: 'assistant' }] });
    });

    expect(onEvent).toHaveBeenNthCalledWith(
      1,
      { messages: [{ id: 'm1', type: 'assistant' }] },
      true
    );
    expect(onEvent).toHaveBeenNthCalledWith(
      2,
      { messages: [{ id: 'm2', type: 'assistant' }] },
      false
    );

    act(() => {
      eventSource.error();
    });

    expect(result.current.isSessionLive).toBe(false);

    act(() => {
      eventSource.open();
      eventSource.emitMessage({ messages: [{ id: 'm3', type: 'assistant' }] });
    });

    expect(onEvent).toHaveBeenNthCalledWith(
      3,
      { messages: [{ id: 'm3', type: 'assistant' }] },
      true
    );
  });

  it('deduplicates repeated navigate UI actions across reconnect snapshots', () => {
    const onEvent = vi.fn();
    const onNavigate = vi.fn();

    renderHook(() =>
      useSSEConnection('sess-1', '/api/v1', 'local', {
        onEvent,
        onNavigate,
      })
    );

    const eventSource = MockEventSource.instances[0];
    if (!eventSource) {
      throw new Error('expected EventSource instance');
    }

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
      eventSource.open();
      eventSource.emitMessage(snapshot);
      eventSource.error();
      eventSource.open();
      eventSource.emitMessage(snapshot);
    });

    expect(onEvent).toHaveBeenCalledTimes(2);
    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(onNavigate).toHaveBeenCalledWith('/runs/run-1');
  });

  it('ignores messages from a stale connection after switching sessions', () => {
    const onEvent = vi.fn();
    const onNavigate = vi.fn();

    const { rerender } = renderHook(
      ({ sessionId }) =>
        useSSEConnection(sessionId, '/api/v1', 'local', {
          onEvent,
          onNavigate,
        }),
      {
        initialProps: { sessionId: 'sess-1' as string | null },
      }
    );

    const firstEventSource = MockEventSource.instances[0];
    if (!firstEventSource) {
      throw new Error('expected initial EventSource instance');
    }

    rerender({ sessionId: 'sess-2' });

    expect(MockEventSource.instances).toHaveLength(2);

    const secondEventSource = MockEventSource.instances[1];
    if (!secondEventSource) {
      throw new Error('expected replacement EventSource instance');
    }

    act(() => {
      secondEventSource.open();
      secondEventSource.emitMessage({
        messages: [{ id: 'new-msg', type: 'assistant' }],
      });
      firstEventSource.emitMessage({
        messages: [{ id: 'stale-msg', type: 'assistant' }],
      });
      firstEventSource.error();
    });

    expect(onEvent).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith(
      { messages: [{ id: 'new-msg', type: 'assistant' }] },
      true
    );
  });
});
