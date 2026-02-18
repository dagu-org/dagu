import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '@/api/v1/schema';
import { useConfig } from '@/contexts/ConfigContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { useAgentChatContext } from '../context/AgentChatContext';
import { getAuthToken } from '@/lib/authHeaders';
import {
  DAGContext,
  DelegateInfo,
  Message,
  MessageType,
  PromptType,
  SessionWithState,
  StreamResponse,
  UIActionType,
  UserPromptResponse,
} from '../types';

type ApiSessionWithState = components['schemas']['AgentSessionWithState'];
type ApiMessage = components['schemas']['AgentMessage'];
type ApiSessionDetail = components['schemas']['AgentSessionDetailResponse'];

function buildStreamUrl(baseUrl: string, sessionId: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}/sessions/${sessionId}/stream`, window.location.origin);
  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }
  url.searchParams.set('remoteNode', remoteNode);
  return url.toString();
}

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
      tool_use_id: tr.toolCallId,
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

function convertApiSessions(sessions: ApiSessionWithState[]): SessionWithState[] {
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
          model: detail.sessionState.model,
          total_cost: detail.sessionState.totalCost,
        }
      : undefined,
  };
}

function toDagContextsBody(dagContexts?: DAGContext[]) {
  return dagContexts?.map((dc) => ({ dagFile: dc.dag_file, dagRunId: dc.dag_run_id }));
}

const MAX_SSE_RETRIES = 3;

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
    setSessionId,
    setMessages,
    setSessionState,
    setSessions,
    addMessage,
    setPendingUserMessage,
    clearSession,
  } = useAgentChatContext();

  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [answeredPrompts, setAnsweredPrompts] = useState<Record<string, string>>({});
  // delegates[] tracks which DelegatePanels are currently open
  const [delegates, setDelegates] = useState<DelegateInfo[]>([]);
  // delegateStatuses is a persistent lookup for ALL delegates (running + completed)
  // used by ChatMessages for inline status display and task lookup
  const [delegateStatuses, setDelegateStatuses] = useState<Record<string, DelegateInfo>>({});
  // delegateMessages carries messages piped through parent SSE (no separate connections needed)
  const [delegateMessages, setDelegateMessages] = useState<Record<string, Message[]>>({});
  const delegateMessagesRef = useRef<Record<string, Message[]>>({});
  const zIndexCounterRef = useRef(60);
  const baseUrl = `${config.apiURL}/agent`;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const closeEventSource = useCallback((): void => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  useEffect(() => {
    return closeEventSource;
  }, [closeEventSource]);

  useEffect(() => {
    if (!sessionId) {
      closeEventSource();
      return;
    }

    eventSourceRef.current?.close();

    const eventSource = new EventSource(buildStreamUrl(baseUrl, sessionId, remoteNode));
    eventSourceRef.current = eventSource;

    eventSource.onmessage = (event) => {
      try {
        const data: StreamResponse = JSON.parse(event.data);
        retryCountRef.current = 0;

        for (const msg of data.messages ?? []) {
          addMessage(msg);
          if (msg.type === 'ui_action' && msg.ui_action?.type === 'navigate' && msg.ui_action.path) {
            navigate(msg.ui_action.path);
          }
        }

        if (data.session_state) {
          setSessionState(data.session_state);
        }

        if (data.delegate_messages) {
          const { delegate_id, messages: msgs } = data.delegate_messages;
          setDelegateMessages((prev) => {
            const existing = prev[delegate_id] || [];
            const idxMap = new Map<string, number>();
            existing.forEach((m, i) => idxMap.set(m.id, i));
            const updated = [...existing];
            for (const msg of msgs) {
              const idx = idxMap.get(msg.id);
              if (idx !== undefined) {
                updated[idx] = msg;
              } else {
                idxMap.set(msg.id, updated.length);
                updated.push(msg);
              }
            }
            const next = { ...prev, [delegate_id]: updated };
            delegateMessagesRef.current = next;
            return next;
          });
        }

        if (data.delegate_event) {
          const evt = data.delegate_event;
          if (evt.type === 'started') {
            zIndexCounterRef.current++;
            const zIndex = zIndexCounterRef.current;
            setDelegates((prev) => {
              const info: DelegateInfo = {
                id: evt.delegate_id,
                task: evt.task,
                status: 'running',
                zIndex,
                positionIndex: prev.length,
              };
              return [...prev, info];
            });
            setDelegateStatuses((prev) => ({
              ...prev,
              [evt.delegate_id]: {
                id: evt.delegate_id,
                task: evt.task,
                status: 'running',
                zIndex,
                positionIndex: 0,
              },
            }));
          } else if (evt.type === 'completed') {
            // Update status instead of removing — let DelegatePanel play close animation
            setDelegates((prev) => prev.map((d) =>
              d.id === evt.delegate_id ? { ...d, status: 'completed' as const } : d
            ));
            setDelegateStatuses((prev) => ({
              ...prev,
              [evt.delegate_id]: {
                ...prev[evt.delegate_id],
                id: evt.delegate_id,
                task: evt.task,
                status: 'completed',
                zIndex: prev[evt.delegate_id]?.zIndex ?? 60,
                positionIndex: prev[evt.delegate_id]?.positionIndex ?? 0,
              },
            }));
          }
        }
      } catch {
        // SSE parse errors are transient, stream will continue
      }
    };

    eventSource.onerror = () => {
      if (eventSource.readyState === EventSource.CLOSED && retryCountRef.current < MAX_SSE_RETRIES) {
        retryCountRef.current++;
        setTimeout(() => {
          if (sessionId && eventSourceRef.current === eventSource) {
            setSessionId(sessionId);
          }
        }, 1000);
      }
    };

    return () => {
      eventSource.close();
    };
  }, [sessionId, baseUrl, remoteNode, addMessage, setSessionState, setSessionId, navigate, closeEventSource]);

  const startSession = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<string> => {
      const { data, error: apiError } = await client.POST('/agent/sessions', {
        params: { query: { remoteNode } },
        body: {
          message,
          model,
          dagContexts: toDagContextsBody(dagContexts),
          safeMode: preferences.safeMode,
        },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to create session');
      setSessionId(data.sessionId);
      const { data: sessionsData } = await client.GET('/agent/sessions', {
        params: { query: { remoteNode } },
      });
      setSessions(sessionsData ? convertApiSessions(sessionsData) : []);
      return data.sessionId;
    },
    [client, remoteNode, setSessionId, setSessions, preferences.safeMode]
  );

  const sendMessage = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<void> => {
      setIsSending(true);
      setError(null);
      setPendingUserMessage(message);

      try {
        if (!sessionId) {
          await startSession(message, model, dagContexts);
          return;
        }
        const { error: apiError } = await client.POST('/agent/sessions/{sessionId}/chat', {
          params: { path: { sessionId }, query: { remoteNode } },
          body: {
            message,
            model,
            dagContexts: toDagContextsBody(dagContexts),
            safeMode: preferences.safeMode,
          },
        });
        if (apiError) throw new Error(apiError.message || 'Failed to send message');
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to send message');
        setPendingUserMessage(null);
        throw err;
      } finally {
        setIsSending(false);
      }
    },
    [client, remoteNode, sessionId, startSession, setPendingUserMessage, preferences.safeMode]
  );

  const cancelSession = useCallback(async (): Promise<void> => {
    if (!sessionId) return;
    const { error: apiError } = await client.POST('/agent/sessions/{sessionId}/cancel', {
      params: { path: { sessionId }, query: { remoteNode } },
    });
    if (apiError) throw new Error(apiError.message || 'Failed to cancel session');
  }, [client, remoteNode, sessionId]);

  const respondToPrompt = useCallback(async (response: UserPromptResponse, displayValue: string): Promise<void> => {
    if (!sessionId) return;

    try {
      const { error: apiError } = await client.POST('/agent/sessions/{sessionId}/respond', {
        params: { path: { sessionId }, query: { remoteNode } },
        body: {
          promptId: response.prompt_id,
          selectedOptionIds: response.selected_option_ids,
          freeTextResponse: response.free_text_response,
          cancelled: response.cancelled,
        },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to submit response');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit response');
    } finally {
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
    }
  }, [client, remoteNode, sessionId]);

  const fetchSessions = useCallback(async (): Promise<void> => {
    try {
      const { data, error: apiError } = await client.GET('/agent/sessions', {
        params: { query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to fetch sessions');
      setSessions(data ? convertApiSessions(data) : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch sessions');
      setSessions([]);
    }
  }, [client, remoteNode, setSessions]);

  const selectSession = useCallback(
    async (id: string): Promise<void> => {
      const { data, error: apiError } = await client.GET('/agent/sessions/{sessionId}', {
        params: { path: { sessionId: id }, query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to load session');
      const converted = convertApiSessionDetail(data);
      setSessionId(id);
      setMessages(converted.messages || []);
      setAnsweredPrompts({});
      setDelegates([]);
      setDelegateStatuses({});
      setDelegateMessages({});
      delegateMessagesRef.current = {};
      if (converted.session_state) {
        setSessionState(converted.session_state);
      }
    },
    [client, remoteNode, setSessionId, setMessages, setSessionState]
  );

  const isWorking = isSending || sessionState?.working || false;

  const clearError = useCallback(() => setError(null), []);

  const handleClearSession = useCallback(() => {
    clearSession();
    setAnsweredPrompts({});
    setDelegates([]);
    setDelegateStatuses({});
    setDelegateMessages({});
    delegateMessagesRef.current = {};
  }, [clearSession]);

  const bringToFront = useCallback((delegateId: string) => {
    zIndexCounterRef.current++;
    setDelegates((prev) => prev.map((d) =>
      d.id === delegateId ? { ...d, zIndex: zIndexCounterRef.current } : d
    ));
  }, []);

  const reopenDelegate = useCallback(async (delegateId: string, task: string) => {
    // Fetch messages from REST API if not already piped
    if (!delegateMessagesRef.current[delegateId]?.length) {
      try {
        const { data } = await client.GET('/agent/sessions/{sessionId}', {
          params: { path: { sessionId: delegateId }, query: { remoteNode } },
        });
        if (data) {
          const converted = convertApiSessionDetail(data);
          const msgs = converted.messages || [];
          setDelegateMessages((prev) => {
            const next = { ...prev, [delegateId]: msgs };
            delegateMessagesRef.current = next;
            return next;
          });
        }
      } catch {
        // Best effort — panel will show empty state
      }
    }
    zIndexCounterRef.current++;
    setDelegates((prev) => {
      if (prev.some((d) => d.id === delegateId)) return prev;
      return [...prev, {
        id: delegateId,
        task,
        status: 'completed' as const,
        zIndex: zIndexCounterRef.current,
        positionIndex: prev.length,
      }];
    });
  }, [client, remoteNode]);

  const removeDelegate = useCallback((delegateId: string) => {
    setDelegates((prev) =>
      prev
        .filter((d) => d.id !== delegateId)
        .map((d, idx) => ({ ...d, positionIndex: idx }))
    );
  }, []);

  return {
    sessionId,
    messages,
    pendingUserMessage,
    sessionState,
    sessions,
    isWorking,
    error,
    answeredPrompts,
    setError,
    sendMessage,
    cancelSession,
    clearSession: handleClearSession,
    clearError,
    fetchSessions,
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
