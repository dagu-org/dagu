import {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '@/api/v1/schema';
import { useConfig } from '@/contexts/ConfigContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { useAgentChatContext } from '../context/AgentChatContext';
import {
  DAGContext,
  DelegateStatus,
  Message,
  MessageType,
  PromptType,
  SessionWithState,
  StreamResponse,
  UIActionType,
  UserPromptResponse,
} from '../types';
import { useSSEConnection } from './useSSEConnection';
import { useDelegateManager } from './useDelegateManager';

type ApiSessionWithState = components['schemas']['AgentSessionWithState'];
type ApiMessage = components['schemas']['AgentMessage'];
type ApiSessionDetail = components['schemas']['AgentSessionDetailResponse'];
const FALLBACK_POLL_INTERVAL_MS = 2000;

function convertApiMessage(msg: ApiMessage): Message {
  return {
    id: msg.id,
    session_id: msg.sessionId,
    type: msg.type as MessageType,
    sequence_id: msg.sequenceId,
    content: msg.content,
    tool_calls: msg.toolCalls?.map((tc) => ({
      id: tc.id,
      type: tc.type,
      function: tc.function,
    })),
    tool_results: msg.toolResults?.map((tr) => ({
      tool_call_id: tr.toolCallId,
      content: tr.content,
      is_error: tr.isError,
    })),
    ui_action: msg.uiAction?.type
      ? { type: msg.uiAction.type as UIActionType, path: msg.uiAction.path }
      : undefined,
    user_prompt: msg.userPrompt
      ? {
          prompt_id: msg.userPrompt.promptId,
          question: msg.userPrompt.question,
          options: msg.userPrompt.options,
          allow_free_text: msg.userPrompt.allowFreeText,
          free_text_placeholder: msg.userPrompt.freeTextPlaceholder,
          multi_select: msg.userPrompt.multiSelect,
          prompt_type: msg.userPrompt.promptType as PromptType | undefined,
          command: msg.userPrompt.command,
          working_dir: msg.userPrompt.workingDir,
        }
      : undefined,
    usage: msg.usage
      ? {
          prompt_tokens: msg.usage.promptTokens ?? 0,
          completion_tokens: msg.usage.completionTokens ?? 0,
          total_tokens: msg.usage.totalTokens ?? 0,
        }
      : undefined,
    cost: msg.cost,
    delegate_ids: msg.delegateIds ?? undefined,
    created_at: msg.createdAt,
  };
}

function convertApiSessions(
  sessions: ApiSessionWithState[]
): SessionWithState[] {
  return sessions.map((s) => ({
    session: {
      id: s.session.id,
      user_id: s.session.userId,
      title: s.session.title,
      created_at: s.session.createdAt,
      updated_at: s.session.updatedAt,
      parent_session_id: s.session.parentSessionId,
      delegate_task: s.session.delegateTask,
    },
    working: s.working,
    has_pending_prompt: s.hasPendingPrompt,
    model: s.model,
    total_cost: s.totalCost,
  }));
}

function convertApiSessionDetail(detail: ApiSessionDetail): StreamResponse {
  return {
    messages: detail.messages?.map(convertApiMessage),
    session: detail.session
      ? {
          id: detail.session.id,
          user_id: detail.session.userId,
          title: detail.session.title,
          created_at: detail.session.createdAt,
          updated_at: detail.session.updatedAt,
          parent_session_id: detail.session.parentSessionId,
          delegate_task: detail.session.delegateTask,
        }
      : undefined,
    session_state: detail.sessionState
      ? {
          session_id: detail.sessionState.sessionId,
          working: detail.sessionState.working,
          has_pending_prompt: detail.sessionState.hasPendingPrompt,
          model: detail.sessionState.model,
          total_cost: detail.sessionState.totalCost,
        }
      : undefined,
    delegates: detail.delegates?.map((d) => ({
      id: d.id,
      task: d.task,
      status: d.status as DelegateStatus,
      cost: d.cost,
    })),
  };
}

function toDagContextsBody(dagContexts?: DAGContext[]) {
  return dagContexts?.map((dc) => ({
    dagFile: dc.dag_file,
    dagRunId: dc.dag_run_id,
  }));
}

function mergeMessages(current: Message[], incoming: Message[]): Message[] {
  if (incoming.length === 0) {
    return current;
  }

  const next = [...current];
  const indexById = new Map<string, number>();
  next.forEach((message, index) => {
    indexById.set(message.id, index);
  });

  let requiresSort = false;
  let lastAppendedSequence =
    current.length > 0
      ? current[current.length - 1]!.sequence_id
      : Number.NEGATIVE_INFINITY;

  for (const message of incoming) {
    const existingIndex = indexById.get(message.id);
    if (existingIndex === undefined) {
      indexById.set(message.id, next.length);
      next.push(message);
      if (message.sequence_id < lastAppendedSequence) {
        requiresSort = true;
      }
      lastAppendedSequence = message.sequence_id;
      continue;
    }

    const previousSequence =
      existingIndex > 0
        ? next[existingIndex - 1]!.sequence_id
        : Number.NEGATIVE_INFINITY;
    const followingSequence =
      existingIndex < next.length - 1
        ? next[existingIndex + 1]!.sequence_id
        : Number.POSITIVE_INFINITY;
    next[existingIndex] = message;
    if (
      message.sequence_id < previousSequence ||
      message.sequence_id > followingSequence
    ) {
      requiresSort = true;
    }
  }

  if (!requiresSort) {
    return next;
  }

  return next.sort((left, right) => left.sequence_id - right.sequence_id);
}

export function useAgentChat() {
  const config = useConfig();
  const client = useClient();
  const navigate = useNavigate();
  const { preferences } = useUserPreferences();
  const appBarContext = useContext(AppBarContext);
  const {
    sessionId,
    messages,
    pendingUserMessage,
    sessionState,
    sessions,
    hasMoreSessions,
    sessionPage,
    setSessionId,
    setMessages,
    setSessionState,
    setSessions,
    appendSessions,
    setHasMoreSessions,
    setSessionPage,
    setPendingUserMessage,
    clearSession,
  } = useAgentChatContext();

  const selectGenRef = useRef(0);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [answeredPrompts, setAnsweredPrompts] = useState<
    Record<string, string>
  >({});

  const apiURL = config.apiURL;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const dm = useDelegateManager();
  const {
    delegates,
    delegateStatuses,
    delegateMessages,
    reconcileDelegateSnapshots,
    applyDelegateSessionSnapshot,
    handleDelegateMessages,
    handleDelegateEvent,
    resetDelegates,
    bringToFront,
    openDelegate,
    setDelegateMessagesForId,
    hasDelegateMessages,
    removeDelegate,
  } = dm;
  const openDelegateSessionIds = delegates.map((delegate) => delegate.id);
  const sortedOpenDelegateSessionIds = useMemo(
    () => [...openDelegateSessionIds].sort(),
    [openDelegateSessionIds]
  );

  const applySessionSnapshot = useCallback((snapshot: StreamResponse) => {
    const nextMessages = snapshot.messages || [];
    if (
      pendingUserMessage &&
      nextMessages.some(
        (message) =>
          message.type === 'user' && message.content === pendingUserMessage
      )
    ) {
      setPendingUserMessage(null);
    }

    setMessages(nextMessages);
    if (snapshot.session_state) {
      setSessionState(snapshot.session_state);
    }

    reconcileDelegateSnapshots(snapshot.delegates || []);
  }, [
    pendingUserMessage,
    setPendingUserMessage,
    setMessages,
    setSessionState,
    reconcileDelegateSnapshots,
  ]);

  const applySessionEvent = useCallback((event: StreamResponse, replace = false) => {
    if (replace) {
      applySessionSnapshot(event);
      return;
    }

    if (event.messages && event.messages.length > 0) {
      if (
        pendingUserMessage &&
        event.messages.some(
          (message) =>
            message.type === 'user' && message.content === pendingUserMessage
        )
      ) {
        setPendingUserMessage(null);
      }

      setMessages((current) => mergeMessages(current, event.messages || []));
    }

    if (event.session_state) {
      setSessionState(event.session_state);
    }

    if (event.delegates) {
      reconcileDelegateSnapshots(event.delegates);
    }

    if (event.delegate_event) {
      handleDelegateEvent(event.delegate_event);
    }

    if (event.delegate_messages) {
      handleDelegateMessages(event.delegate_messages);
    }
  }, [
    applySessionSnapshot,
    pendingUserMessage,
    setPendingUserMessage,
    setMessages,
    setSessionState,
    reconcileDelegateSnapshots,
    handleDelegateEvent,
    handleDelegateMessages,
  ]);

  const delegateStatusesRef = useRef(delegateStatuses);
  delegateStatusesRef.current = delegateStatuses;

  const applyDelegateSnapshot = useCallback(
    (delegateId: string, snapshot: StreamResponse) => {
      const existing = delegateStatusesRef.current[delegateId];
      applyDelegateSessionSnapshot(
        delegateId,
        snapshot.session?.delegate_task || existing?.task || '',
        snapshot.session_state?.working ? 'running' : 'completed',
        snapshot.messages || []
      );
    },
    [applyDelegateSessionSnapshot]
  );

  const applySessionSnapshotRef = useRef(applySessionSnapshot);
  applySessionSnapshotRef.current = applySessionSnapshot;

  const applyDelegateSnapshotRef = useRef(applyDelegateSnapshot);
  applyDelegateSnapshotRef.current = applyDelegateSnapshot;

  const fetchSessionDetail = useCallback(
    async (id: string): Promise<StreamResponse> => {
      const { data, error: apiError } = await client.GET(
        '/agent/sessions/{sessionId}',
        {
          params: { path: { sessionId: id }, query: { remoteNode } },
        }
      );
      if (apiError)
        throw new Error(apiError.message || 'Failed to load session');
      if (!data) throw new Error('Failed to load session');
      return convertApiSessionDetail(data);
    },
    [client, remoteNode]
  );

  const fetchSessionDetailRef = useRef(fetchSessionDetail);
  fetchSessionDetailRef.current = fetchSessionDetail;

  const sseStatus = useSSEConnection(sessionId, apiURL, remoteNode, {
    onEvent: (event, replace) => {
      applySessionEvent(event, replace);
    },
    onNavigate: (path) => navigate(path),
  });

  useEffect(() => {
    const shouldPollSession = !!sessionId && !sseStatus.isSessionLive;
    if (!shouldPollSession) {
      return;
    }

    let active = true;
    let nextPollTimeout: ReturnType<typeof setTimeout> | null = null;

    const scheduleNextPoll = () => {
      if (!active) {
        return;
      }
      nextPollTimeout = setTimeout(() => {
        void runPollLoop();
      }, FALLBACK_POLL_INTERVAL_MS);
    };

    const runPollLoop = async () => {
      if (sessionId) {
        try {
          const snapshot = await fetchSessionDetailRef.current(sessionId);
          if (!active) {
            return;
          }
          applySessionSnapshotRef.current(snapshot);
        } catch {
          // Best effort — the next poll or SSE recovery can still heal state.
        }
      }

      for (const delegateId of sortedOpenDelegateSessionIds) {
        try {
          const snapshot = await fetchSessionDetailRef.current(delegateId);
          if (!active) {
            return;
          }
          applyDelegateSnapshotRef.current(delegateId, snapshot);
        } catch {
          // Best effort — the next poll or SSE recovery can still heal state.
        }
      }

      scheduleNextPoll();
    };

    scheduleNextPoll();

    return () => {
      active = false;
      if (nextPollTimeout) {
        clearTimeout(nextPollTimeout);
      }
    };
  }, [sessionId, sseStatus.isSessionLive, sortedOpenDelegateSessionIds]);

  const fetchSessionsPage = useCallback(
    async (page: number): Promise<void> => {
      try {
        const { data, error: apiError } = await client.GET('/agent/sessions', {
          params: { query: { remoteNode, page, perPage: 30 } },
        });
        if (apiError)
          throw new Error(apiError.message || 'Failed to fetch sessions');
        if (!data) return;

        const converted = convertApiSessions(data.sessions);
        if (page === 1) {
          setSessions(converted);
        } else {
          appendSessions(converted);
        }
        setHasMoreSessions(
          data.pagination.currentPage < data.pagination.totalPages
        );
        setSessionPage(page);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to fetch sessions'
        );
        if (page === 1) setSessions([]);
      }
    },
    [
      client,
      remoteNode,
      setSessions,
      appendSessions,
      setHasMoreSessions,
      setSessionPage,
    ]
  );

  const loadMoreSessions = useCallback(async (): Promise<void> => {
    if (!hasMoreSessions) return;
    await fetchSessionsPage(sessionPage + 1);
  }, [fetchSessionsPage, sessionPage, hasMoreSessions]);

  const startSession = useCallback(
    async (
      message: string,
      model?: string,
      dagContexts?: DAGContext[],
      soulId?: string
    ): Promise<string> => {
      const { data, error: apiError } = await client.POST('/agent/sessions', {
        params: { query: { remoteNode } },
        body: {
          message,
          model,
          dagContexts: toDagContextsBody(dagContexts),
          safeMode: preferences.safeMode,
          soulId: soulId || undefined,
        },
      });
      if (apiError)
        throw new Error(apiError.message || 'Failed to create session');
      setSessionId(data.sessionId);
      await fetchSessionsPage(1);
      return data.sessionId;
    },
    [client, remoteNode, setSessionId, fetchSessionsPage, preferences.safeMode]
  );

  const sendMessage = useCallback(
    async (
      message: string,
      model?: string,
      dagContexts?: DAGContext[],
      soulId?: string
    ): Promise<void> => {
      setIsSending(true);
      setError(null);
      setPendingUserMessage(message);

      try {
        if (!sessionId) {
          await startSession(message, model, dagContexts, soulId);
          return;
        }
        const { error: apiError } = await client.POST(
          '/agent/sessions/{sessionId}/chat',
          {
            params: { path: { sessionId }, query: { remoteNode } },
            body: {
              message,
              model,
              dagContexts: toDagContextsBody(dagContexts),
              safeMode: preferences.safeMode,
              soulId: soulId || undefined,
            },
          }
        );
        if (apiError)
          throw new Error(apiError.message || 'Failed to send message');
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to send message');
        setPendingUserMessage(null);
        throw err;
      } finally {
        setIsSending(false);
      }
    },
    [
      client,
      remoteNode,
      sessionId,
      startSession,
      setPendingUserMessage,
      preferences.safeMode,
    ]
  );

  const cancelSession = useCallback(async (): Promise<void> => {
    if (!sessionId) return;
    const { error: apiError } = await client.POST(
      '/agent/sessions/{sessionId}/cancel',
      {
        params: { path: { sessionId }, query: { remoteNode } },
      }
    );
    if (apiError)
      throw new Error(apiError.message || 'Failed to cancel session');
  }, [client, remoteNode, sessionId]);

  const respondToPrompt = useCallback(
    async (
      response: UserPromptResponse,
      displayValue: string
    ): Promise<void> => {
      if (!sessionId) return;

      try {
        const { error: apiError } = await client.POST(
          '/agent/sessions/{sessionId}/respond',
          {
            params: { path: { sessionId }, query: { remoteNode } },
            body: {
              promptId: response.prompt_id,
              selectedOptionIds: response.selected_option_ids,
              freeTextResponse: response.free_text_response,
              cancelled: response.cancelled,
            },
          }
        );
        if (apiError)
          throw new Error(apiError.message || 'Failed to submit response');
        setAnsweredPrompts((prev) => ({
          ...prev,
          [response.prompt_id]: displayValue,
        }));
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to submit response'
        );
      }
    },
    [client, remoteNode, sessionId]
  );

  const fetchSessions = useCallback(async (): Promise<void> => {
    await fetchSessionsPage(1);
  }, [fetchSessionsPage]);

  const selectSession = useCallback(
    async (id: string): Promise<void> => {
      const gen = ++selectGenRef.current;
      const converted = await fetchSessionDetail(id);
      if (gen !== selectGenRef.current) return;
      setSessionId(id);
      setAnsweredPrompts({});
      applySessionSnapshot(converted);
    },
    [fetchSessionDetail, setSessionId, applySessionSnapshot]
  );

  const isWorking = isSending || sessionState?.working || false;

  const clearError = useCallback(() => setError(null), []);

  const handleClearSession = useCallback(() => {
    selectGenRef.current++;
    clearSession();
    setAnsweredPrompts({});
    resetDelegates();
  }, [clearSession, resetDelegates]);

  const reopenDelegate = useCallback(
    async (delegateId: string, task: string) => {
      if (!hasDelegateMessages(delegateId)) {
        try {
          const snapshot = await fetchSessionDetail(delegateId);
          setDelegateMessagesForId(delegateId, task, snapshot.messages || []);
        } catch {
          // Best effort — panel will show empty state
        }
      }
      openDelegate(delegateId, task);
    },
    [
      fetchSessionDetail,
      hasDelegateMessages,
      setDelegateMessagesForId,
      openDelegate,
    ]
  );

  return {
    sessionId,
    messages,
    pendingUserMessage,
    sessionState,
    sessions,
    hasMoreSessions,
    isWorking,
    error,
    answeredPrompts,
    setError,
    sendMessage,
    cancelSession,
    clearSession: handleClearSession,
    clearError,
    fetchSessions,
    loadMoreSessions,
    selectSession,
    respondToPrompt,
    delegates,
    delegateStatuses,
    delegateMessages,
    bringToFront,
    reopenDelegate,
    removeDelegate,
  };
}
