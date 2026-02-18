import { useCallback, useContext, useRef, useState } from 'react';
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
    delegates: detail.delegates?.map((d) => ({
      id: d.id,
      task: d.task,
      status: d.status as DelegateStatus,
      cost: d.cost,
    })),
  };
}

function toDagContextsBody(dagContexts?: DAGContext[]) {
  return dagContexts?.map((dc) => ({ dagFile: dc.dag_file, dagRunId: dc.dag_run_id }));
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
    setSessionId,
    setMessages,
    setSessionState,
    setSessions,
    addMessage,
    setPendingUserMessage,
    clearSession,
  } = useAgentChatContext();

  const selectGenRef = useRef(0);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [answeredPrompts, setAnsweredPrompts] = useState<Record<string, string>>({});

  const baseUrl = `${config.apiURL}/agent`;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const dm = useDelegateManager();

  useSSEConnection(sessionId, baseUrl, remoteNode, {
    onMessage: addMessage,
    onSessionState: setSessionState,
    onDelegateSnapshots: dm.handleDelegateSnapshots,
    onDelegateMessages: dm.handleDelegateMessages,
    onDelegateEvent: dm.handleDelegateEvent,
    onNavigate: (path) => navigate(path),
    onPreConnect: dm.resetDelegates,
  });

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
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit response');
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
      const gen = ++selectGenRef.current;
      const { data, error: apiError } = await client.GET('/agent/sessions/{sessionId}', {
        params: { path: { sessionId: id }, query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to load session');
      if (gen !== selectGenRef.current) return;
      const converted = convertApiSessionDetail(data);
      setSessionId(id);
      setMessages(converted.messages || []);
      setAnsweredPrompts({});
      dm.restoreDelegates(converted.delegates || []);
      if (converted.session_state) {
        setSessionState(converted.session_state);
      }
    },
    [client, remoteNode, setSessionId, setMessages, setSessionState, dm]
  );

  const isWorking = isSending || sessionState?.working || false;

  const clearError = useCallback(() => setError(null), []);

  const handleClearSession = useCallback(() => {
    selectGenRef.current++;
    clearSession();
    setAnsweredPrompts({});
    dm.resetDelegates();
  }, [clearSession, dm]);

  const reopenDelegate = useCallback(async (delegateId: string, task: string) => {
    if (!dm.hasDelegateMessages(delegateId)) {
      try {
        const { data } = await client.GET('/agent/sessions/{sessionId}', {
          params: { path: { sessionId: delegateId }, query: { remoteNode } },
        });
        if (data) {
          const msgs = convertApiSessionDetail(data).messages || [];
          dm.setDelegateMessagesForId(delegateId, task, msgs);
        }
      } catch {
        // Best effort — panel will show empty state
      }
    }
    dm.openDelegate(delegateId, task);
  }, [client, remoteNode, dm]);

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
    delegates: dm.delegates,
    delegateStatuses: dm.delegateStatuses,
    delegateMessages: dm.delegateMessages,
    bringToFront: dm.bringToFront,
    reopenDelegate,
    removeDelegate: dm.removeDelegate,
  };
}
