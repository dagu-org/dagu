import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfig } from '@/contexts/ConfigContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useAgentChatContext } from '../context/AgentChatContext';
import { getAuthToken, getAuthHeaders } from '@/lib/authHeaders';
import {
  ChatRequest,
  SessionWithState,
  DAGContext,
  NewSessionResponse,
  StreamResponse,
  UserPromptResponse,
} from '../types';

function buildChatRequest(
  message: string,
  model?: string,
  dagContexts?: DAGContext[],
  safeMode?: boolean
): ChatRequest {
  return { message, model, dag_contexts: dagContexts, safe_mode: safeMode };
}

async function fetchWithAuth<T>(url: string, options?: RequestInit): Promise<T> {
  const hasBody = Boolean(options?.body);
  const headers: Record<string, string> = {
    ...getAuthHeaders(),
    ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
    ...(options?.headers as Record<string, string>),
  };

  const response = await fetch(url, { ...options, headers });
  if (!response.ok) {
    const errorData = await response.json().catch(() => null);
    throw new Error(errorData?.message || response.statusText || 'Request failed');
  }
  return response.json();
}

function buildStreamUrl(baseUrl: string, sessionId: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}/sessions/${sessionId}/stream`, window.location.origin);
  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }
  url.searchParams.set('remoteNode', remoteNode);
  return url.toString();
}

function buildApiUrl(baseUrl: string, path: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}${path}`, window.location.origin);
  url.searchParams.set('remoteNode', remoteNode);
  return url.toString();
}

const MAX_SSE_RETRIES = 3;

export function useAgentChat() {
  const config = useConfig();
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
      const data = await fetchWithAuth<NewSessionResponse>(
        buildApiUrl(baseUrl, '/sessions/new', remoteNode),
        { method: 'POST', body: JSON.stringify(buildChatRequest(message, model, dagContexts, preferences.safeMode)) }
      );
      setSessionId(data.session_id);
      // Refresh sessions list to include the new one
      const sessns = await fetchWithAuth<SessionWithState[]>(buildApiUrl(baseUrl, '/sessions', remoteNode));
      setSessions(sessns || []);
      return data.session_id;
    },
    [baseUrl, remoteNode, setSessionId, setSessions, preferences.safeMode]
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
        await fetchWithAuth(buildApiUrl(baseUrl, `/sessions/${sessionId}/chat`, remoteNode), {
          method: 'POST',
          body: JSON.stringify(buildChatRequest(message, model, dagContexts, preferences.safeMode)),
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to send message');
        setPendingUserMessage(null);
        throw err;
      } finally {
        setIsSending(false);
      }
    },
    [baseUrl, remoteNode, sessionId, startSession, setPendingUserMessage, preferences.safeMode]
  );

  const cancelSession = useCallback(async (): Promise<void> => {
    if (!sessionId) return;
    await fetchWithAuth(buildApiUrl(baseUrl, `/sessions/${sessionId}/cancel`, remoteNode), { method: 'POST' });
  }, [baseUrl, remoteNode, sessionId]);

  const respondToPrompt = useCallback(async (response: UserPromptResponse, displayValue: string): Promise<void> => {
    if (!sessionId) return;

    try {
      await fetchWithAuth(buildApiUrl(baseUrl, `/sessions/${sessionId}/respond`, remoteNode), {
        method: 'POST',
        body: JSON.stringify(response),
      });
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
    } catch (err) {
      // If prompt not found (e.g., after reload), mark as answered anyway to update UI
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
      setError(err instanceof Error ? err.message : 'Failed to submit response');
    }
  }, [baseUrl, remoteNode, sessionId]);

  const fetchSessions = useCallback(async (): Promise<void> => {
    try {
      const data = await fetchWithAuth<SessionWithState[]>(buildApiUrl(baseUrl, '/sessions', remoteNode));
      setSessions(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch sessions');
      setSessions([]);
    }
  }, [baseUrl, remoteNode, setSessions]);

  const selectSession = useCallback(
    async (id: string): Promise<void> => {
      const data = await fetchWithAuth<StreamResponse>(buildApiUrl(baseUrl, `/sessions/${id}`, remoteNode));
      setSessionId(id);
      setMessages(data.messages || []);
      setAnsweredPrompts({}); // Clear answered prompts when switching sessions
      if (data.session_state) {
        setSessionState(data.session_state);
      }
    },
    [baseUrl, remoteNode, setSessionId, setMessages, setSessionState]
  );

  const isWorking = isSending || sessionState?.working || false;

  const clearError = useCallback(() => setError(null), []);

  const handleClearSession = useCallback(() => {
    clearSession();
    setAnsweredPrompts({});
  }, [clearSession]);

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
    startSession,
    sendMessage,
    cancelSession,
    clearSession: handleClearSession,
    clearError,
    fetchSessions,
    selectSession,
    respondToPrompt,
  };
}
