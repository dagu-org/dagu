import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SSEConnectionState } from '../SSEManager';
import {
  buildAgentSessionTopic,
  endpointToTopic,
  SSEManager,
} from '../SSEManager';

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  readonly close = vi.fn();
  onerror: (() => void) | null = null;
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

  emit(type: string, data: unknown, lastEventId = '') {
    const listeners = this.listeners.get(type);
    if (!listeners) {
      return;
    }

    const event = {
      data: JSON.stringify(data),
      lastEventId,
    } as MessageEvent<string>;
    for (const listener of listeners) {
      listener(event);
    }
  }
}

function snapshotState(state: SSEConnectionState): SSEConnectionState {
  return {
    isConnected: state.isConnected,
    isConnecting: state.isConnecting,
    shouldUseFallback: state.shouldUseFallback,
    error: state.error,
  };
}

describe('endpointToTopic', () => {
  it('maps every supported legacy SSE endpoint to a canonical topic', () => {
    const cases: Array<[string, string]> = [
      ['/events/dags?perPage=100&page=1', 'dagslist:page=1&perPage=100'],
      ['/events/dags/example.yaml', 'dag:example.yaml'],
      ['/events/dags/example.yaml/dag-runs', 'daghistory:example.yaml'],
      [
        '/events/dag-runs?page=2&status=running',
        'dagruns:page=2&status=running',
      ],
      ['/events/dag-runs/mydag/run-1', 'dagrun:mydag/run-1'],
      [
        '/events/dag-runs/mydag/run-1/logs?tail=500',
        'dagrunlogs:mydag/run-1?tail=500',
      ],
      [
        '/events/dag-runs/mydag/run-1/logs/steps/build',
        'steplog:mydag/run-1/build',
      ],
      ['/events/queues?status=active&page=3', 'queues:page=3&status=active'],
      ['/events/queues/default/items', 'queueitems:default'],
      ['/events/docs-tree?prefix=guides', 'doctree:prefix=guides'],
      ['/events/docs/runbooks/deploy%20guide', 'doc:runbooks/deploy guide'],
    ];

    for (const [endpoint, topic] of cases) {
      expect(endpointToTopic(endpoint)).toBe(topic);
    }
  });
});

describe('buildAgentSessionTopic', () => {
  it('builds an agent topic key', () => {
    expect(buildAgentSessionTopic('sess-123')).toBe('agent:sess-123');
  });
});

describe('SSEManager', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockEventSource.instances = [];
    vi.stubGlobal('EventSource', MockEventSource);
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('keeps a fresh topic pending until the server acknowledges the subscription', async () => {
    const manager = new SSEManager();
    const primaryStates: SSEConnectionState[] = [];
    const delegateStates: SSEConnectionState[] = [];

    const unsubscribePrimary = manager.subscribeTopic(
      'agent:sess-1',
      'local',
      '/api/v1',
      {
        onData: () => undefined,
        onStateChange: (state) => primaryStates.push(snapshotState(state)),
      }
    );

    expect(MockEventSource.instances).toHaveLength(1);
    const eventSource = MockEventSource.instances[0];
    eventSource.emit('control', {
      sessionID: 'session-1',
      subscribed: ['agent:sess-1'],
    });

    expect(primaryStates.at(-1)).toMatchObject({
      isConnected: true,
      isConnecting: false,
    });

    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        subscribed: ['agent:sess-1', 'agent:delegate-1'],
        errors: [],
      }),
    } as Response);

    const unsubscribeDelegate = manager.subscribeTopic(
      'agent:delegate-1',
      'local',
      '/api/v1',
      {
        onData: () => undefined,
        onStateChange: (state) => delegateStates.push(snapshotState(state)),
      }
    );

    expect(delegateStates.at(-1)).toMatchObject({
      isConnected: false,
      isConnecting: true,
      shouldUseFallback: false,
    });

    await vi.advanceTimersByTimeAsync(200);

    expect(delegateStates.at(-1)).toMatchObject({
      isConnected: true,
      isConnecting: false,
      shouldUseFallback: false,
    });

    unsubscribeDelegate();
    unsubscribePrimary();
  });

  it('leaves rejected topics offline after the mutation response arrives', async () => {
    const manager = new SSEManager();
    const delegateStates: SSEConnectionState[] = [];

    const unsubscribePrimary = manager.subscribeTopic(
      'agent:sess-1',
      'local',
      '/api/v1',
      {
        onData: () => undefined,
        onStateChange: () => undefined,
      }
    );

    const eventSource = MockEventSource.instances[0];
    eventSource.emit('control', {
      sessionID: 'session-1',
      subscribed: ['agent:sess-1'],
    });

    vi.mocked(fetch).mockResolvedValue({
      ok: false,
      status: 403,
      json: async () => ({
        subscribed: ['agent:sess-1'],
        errors: [
          {
            topic: 'agent:delegate-1',
            code: 'unauthorized',
            message: 'forbidden',
          },
        ],
      }),
    } as Response);

    const unsubscribeDelegate = manager.subscribeTopic(
      'agent:delegate-1',
      'local',
      '/api/v1',
      {
        onData: () => undefined,
        onStateChange: (state) => delegateStates.push(snapshotState(state)),
      }
    );

    await vi.advanceTimersByTimeAsync(200);

    expect(delegateStates.at(-1)).toMatchObject({
      isConnected: false,
      isConnecting: false,
      shouldUseFallback: false,
    });

    unsubscribeDelegate();
    unsubscribePrimary();
  });
});
