import type { ReactNode } from 'react';
import { renderHook, act } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { UserPreferencesProvider } from '@/contexts/UserPreference';
import { AgentChatProvider } from '../../context/AgentChatContext';
import { useAgentChat } from '../useAgentChat';

const getMock = vi.fn();
const postMock = vi.fn();
const navigateMock = vi.fn();
const useSSEConnectionMock = vi.fn();
let mockedSSEStatus = {
  isSessionLive: false,
  liveDelegateSessions: {} as Record<string, boolean>,
};

vi.mock('@/hooks/api', () => ({
  useClient: () => ({
    GET: getMock,
    POST: postMock,
  }),
}));

vi.mock('../useSSEConnection', () => ({
  useSSEConnection: (...args: unknown[]) => useSSEConnectionMock(...args),
}));

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
}));

const testConfig: Config = {
  apiURL: '/api/v1',
  basePath: '/',
  title: 'Dagu',
  navbarColor: '#000000',
  tz: 'UTC',
  tzOffsetInSec: 0,
  version: 'test',
  maxDashboardPageLimit: 100,
  remoteNodes: '',
  authMode: 'none',
  setupRequired: false,
  oidcEnabled: false,
  oidcButtonLabel: '',
  terminalEnabled: true,
  gitSyncEnabled: false,
  agentEnabled: true,
  updateAvailable: false,
  latestVersion: '',
  permissions: {
    writeDags: true,
    runDags: true,
  },
  license: {
    valid: true,
    plan: 'community',
    expiry: '',
    features: [],
    gracePeriod: false,
    community: true,
    source: 'test',
    warningCode: '',
  },
  paths: {
    dagsDir: '',
    logDir: '',
    suspendFlagsDir: '',
    adminLogsDir: '',
    baseConfig: '',
    dagRunsDir: '',
    queueDir: '',
    procDir: '',
    serviceRegistryDir: '',
    configFileUsed: '',
    gitSyncDir: '',
    auditLogsDir: '',
  },
};

function makeApiMessage(id: string, content: string) {
  return {
    id,
    sessionId: 'sess-1',
    type: 'assistant',
    sequenceId: 1,
    content,
    createdAt: '2026-03-13T00:00:00Z',
  };
}

function makeSessionDetailResponse(options?: {
  id?: string;
  delegateTask?: string;
  messages?: Array<ReturnType<typeof makeApiMessage>>;
  working?: boolean;
  delegates?: Array<{
    id: string;
    task: string;
    status: 'running' | 'completed';
  }>;
}) {
  const id = options?.id ?? 'sess-1';
  return {
    session: {
      id,
      title: `Session ${id}`,
      createdAt: '2026-03-13T00:00:00Z',
      updatedAt: '2026-03-13T00:00:00Z',
      delegateTask: options?.delegateTask,
    },
    sessionState: {
      sessionId: id,
      working: options?.working ?? false,
      hasPendingPrompt: false,
      model: 'gpt-test',
      totalCost: 0,
    },
    messages: options?.messages ?? [],
    delegates: options?.delegates ?? [],
  };
}

function TestProviders({ children }: { children: ReactNode }) {
  return (
    <ConfigContext.Provider value={testConfig}>
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: () => undefined,
          remoteNodes: ['local'],
          setRemoteNodes: () => undefined,
          selectedRemoteNode: 'local',
          selectRemoteNode: () => undefined,
        }}
      >
        <UserPreferencesProvider>
          <AgentChatProvider>{children}</AgentChatProvider>
        </UserPreferencesProvider>
      </AppBarContext.Provider>
    </ConfigContext.Provider>
  );
}

describe('useAgentChat fallback polling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    getMock.mockReset();
    postMock.mockReset();
    navigateMock.mockReset();
    mockedSSEStatus = {
      isSessionLive: false,
      liveDelegateSessions: {},
    };
    useSSEConnectionMock.mockReset();
    useSSEConnectionMock.mockImplementation(() => mockedSSEStatus);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls the selected session while multiplex SSE is offline and stops after reconnect', async () => {
    const sessionResponses = [
      makeSessionDetailResponse({
        messages: [makeApiMessage('msg-1', 'initial snapshot')],
        working: false,
      }),
      makeSessionDetailResponse({
        messages: [makeApiMessage('msg-2', 'polled snapshot')],
        working: true,
      }),
    ];
    let sessionFetchCount = 0;

    getMock.mockImplementation(
      async (
        _path: string,
        request?: { params?: { path?: { sessionId?: string } } }
      ) => {
        if (request?.params?.path?.sessionId === 'sess-1') {
          const idx = Math.min(sessionFetchCount, sessionResponses.length - 1);
          sessionFetchCount += 1;
          return { data: sessionResponses[idx] };
        }
        throw new Error('unexpected request');
      }
    );

    const { result, rerender } = renderHook(() => useAgentChat(), {
      wrapper: TestProviders,
    });

    await act(async () => {
      await result.current.selectSession('sess-1');
    });

    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]?.content).toBe('initial snapshot');
    expect(result.current.sessionState?.working).toBe(false);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });

    expect(sessionFetchCount).toBe(2);
    expect(result.current.messages[0]?.content).toBe('polled snapshot');
    expect(result.current.sessionState?.working).toBe(true);

    mockedSSEStatus = {
      isSessionLive: true,
      liveDelegateSessions: {},
    };
    rerender();
    const callsAfterReconnect = sessionFetchCount;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });

    expect(sessionFetchCount).toBe(callsAfterReconnect);
  });

  it('polls open delegate panes while their topic is offline and stops after delegate SSE recovers', async () => {
    mockedSSEStatus = {
      isSessionLive: true,
      liveDelegateSessions: {
        'delegate-1': false,
      },
    };
    useSSEConnectionMock.mockImplementation(() => mockedSSEStatus);

    let delegateFetchCount = 0;
    getMock.mockImplementation(
      async (
        _path: string,
        request?: { params?: { path?: { sessionId?: string } } }
      ) => {
        const sessionId = request?.params?.path?.sessionId;
        if (sessionId === 'sess-1') {
          return {
            data: makeSessionDetailResponse({
              messages: [makeApiMessage('msg-root', 'root session')],
              delegates: [
                { id: 'delegate-1', task: 'Delegate task', status: 'running' },
              ],
            }),
          };
        }
        if (sessionId === 'delegate-1') {
          delegateFetchCount += 1;
          return {
            data: makeSessionDetailResponse({
              id: 'delegate-1',
              delegateTask: 'Delegate task',
              messages: [
                makeApiMessage(
                  `delegate-${delegateFetchCount}`,
                  delegateFetchCount === 1
                    ? 'delegate initial'
                    : 'delegate polled'
                ),
              ],
              working: delegateFetchCount === 1,
            }),
          };
        }
        throw new Error('unexpected request');
      }
    );

    const { result, rerender } = renderHook(() => useAgentChat(), {
      wrapper: TestProviders,
    });

    await act(async () => {
      await result.current.selectSession('sess-1');
    });

    await act(async () => {
      await result.current.reopenDelegate('delegate-1', 'Delegate task');
    });

    expect(result.current.delegateMessages['delegate-1']?.[0]?.content).toBe(
      'delegate initial'
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });

    expect(delegateFetchCount).toBe(2);
    expect(result.current.delegateMessages['delegate-1']?.[0]?.content).toBe(
      'delegate polled'
    );

    mockedSSEStatus = {
      isSessionLive: true,
      liveDelegateSessions: {
        'delegate-1': true,
      },
    };
    rerender();
    const callsAfterReconnect = delegateFetchCount;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });

    expect(delegateFetchCount).toBe(callsAfterReconnect);
  });
});
