import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfig } from '@/contexts/ConfigContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { useAgentChatContext } from '../context/AgentChatContext';
import { getAuthToken, getAuthHeaders } from '@/lib/authHeaders';
import {
  ChatRequest,
  ConversationWithState,
  DAGContext,
  NewConversationResponse,
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

function buildStreamUrl(baseUrl: string, conversationId: string): string {
  const url = new URL(`${baseUrl}/conversations/${conversationId}/stream`, window.location.origin);
  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }
  return url.toString();
}

const MAX_SSE_RETRIES = 3;

export function useAgentChat() {
  const config = useConfig();
  const navigate = useNavigate();
  const { preferences } = useUserPreferences();
  const {
    conversationId,
    messages,
    pendingUserMessage,
    conversationState,
    conversations,
    setConversationId,
    setMessages,
    setConversationState,
    setConversations,
    addMessage,
    setPendingUserMessage,
    clearConversation,
  } = useAgentChatContext();

  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [answeredPrompts, setAnsweredPrompts] = useState<Record<string, string>>({});
  const baseUrl = `${config.apiURL}/agent`;

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
    if (!conversationId) {
      closeEventSource();
      return;
    }

    eventSourceRef.current?.close();

    const eventSource = new EventSource(buildStreamUrl(baseUrl, conversationId));
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

        if (data.conversation_state) {
          setConversationState(data.conversation_state);
        }
      } catch {
        // SSE parse errors are transient, stream will continue
      }
    };

    eventSource.onerror = () => {
      if (eventSource.readyState === EventSource.CLOSED && retryCountRef.current < MAX_SSE_RETRIES) {
        retryCountRef.current++;
        setTimeout(() => {
          if (conversationId && eventSourceRef.current === eventSource) {
            setConversationId(conversationId);
          }
        }, 1000);
      }
    };

    return () => {
      eventSource.close();
    };
  }, [conversationId, baseUrl, addMessage, setConversationState, setConversationId, navigate, closeEventSource]);

  const startConversation = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<string> => {
      const data = await fetchWithAuth<NewConversationResponse>(
        `${baseUrl}/conversations/new`,
        { method: 'POST', body: JSON.stringify(buildChatRequest(message, model, dagContexts, preferences.safeMode)) }
      );
      setConversationId(data.conversation_id);
      // Refresh conversations list to include the new one
      const convs = await fetchWithAuth<ConversationWithState[]>(`${baseUrl}/conversations`);
      setConversations(convs || []);
      return data.conversation_id;
    },
    [baseUrl, setConversationId, setConversations, preferences.safeMode]
  );

  const sendMessage = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<void> => {
      setIsSending(true);
      setError(null);
      setPendingUserMessage(message);

      try {
        if (!conversationId) {
          await startConversation(message, model, dagContexts);
          return;
        }
        await fetchWithAuth(`${baseUrl}/conversations/${conversationId}/chat`, {
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
    [baseUrl, conversationId, startConversation, setPendingUserMessage, preferences.safeMode]
  );

  const cancelConversation = useCallback(async (): Promise<void> => {
    if (!conversationId) return;
    await fetchWithAuth(`${baseUrl}/conversations/${conversationId}/cancel`, { method: 'POST' });
  }, [baseUrl, conversationId]);

  const respondToPrompt = useCallback(async (response: UserPromptResponse, displayValue: string): Promise<void> => {
    if (!conversationId) return;

    try {
      await fetchWithAuth(`${baseUrl}/conversations/${conversationId}/respond`, {
        method: 'POST',
        body: JSON.stringify(response),
      });
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
    } catch (err) {
      // If prompt not found (e.g., after reload), mark as answered anyway to update UI
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
      setError(err instanceof Error ? err.message : 'Failed to submit response');
    }
  }, [baseUrl, conversationId]);

  const fetchConversations = useCallback(async (): Promise<void> => {
    try {
      const data = await fetchWithAuth<ConversationWithState[]>(`${baseUrl}/conversations`);
      setConversations(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch conversations');
      setConversations([]);
    }
  }, [baseUrl, setConversations]);

  const selectConversation = useCallback(
    async (id: string): Promise<void> => {
      const data = await fetchWithAuth<StreamResponse>(`${baseUrl}/conversations/${id}`);
      setConversationId(id);
      setMessages(data.messages || []);
      setAnsweredPrompts({}); // Clear answered prompts when switching conversations
      if (data.conversation_state) {
        setConversationState(data.conversation_state);
      }
    },
    [baseUrl, setConversationId, setMessages, setConversationState]
  );

  const isWorking = isSending || conversationState?.working || false;

  const clearError = useCallback(() => setError(null), []);

  const handleClearConversation = useCallback(() => {
    clearConversation();
    setAnsweredPrompts({});
  }, [clearConversation]);

  return {
    conversationId,
    messages,
    pendingUserMessage,
    conversationState,
    conversations,
    isWorking,
    error,
    answeredPrompts,
    setError,
    startConversation,
    sendMessage,
    cancelConversation,
    clearConversation: handleClearConversation,
    clearError,
    fetchConversations,
    selectConversation,
    respondToPrompt,
  };
}
