import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfig } from '@/contexts/ConfigContext';
import { TOKEN_KEY } from '@/contexts/AuthContext';
import { useAgentChatContext } from '../context/AgentChatContext';
import {
  ChatRequest,
  ConversationWithState,
  DAGContext,
  NewConversationResponse,
  StreamResponse,
} from '../types';

function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

function getAuthHeaders(): Record<string, string> {
  const token = getAuthToken();
  return token
    ? { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }
    : { 'Content-Type': 'application/json' };
}

function buildChatRequest(
  message: string,
  model?: string,
  dagContexts?: DAGContext[]
): ChatRequest {
  return { message, model, dag_contexts: dagContexts };
}

async function fetchWithAuth<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    headers: { ...getAuthHeaders(), ...options?.headers },
  });
  if (!response.ok) {
    let errorMessage = response.statusText || 'Request failed';
    try {
      const errorData = await response.json();
      if (errorData.message) {
        errorMessage = errorData.message;
      }
    } catch {
      // Ignore JSON parse errors
    }
    throw new Error(errorMessage);
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
  const {
    conversationId,
    messages,
    conversationState,
    conversations,
    setConversationId,
    setMessages,
    setConversationState,
    setConversations,
    addMessage,
    clearConversation,
  } = useAgentChatContext();

  const eventSourceRef = useRef<EventSource | null>(null);
  const retryCountRef = useRef(0);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
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
        retryCountRef.current = 0; // Reset retry count on successful message

        data.messages?.forEach((msg) => {
          addMessage(msg);
          if (msg.type === 'ui_action' && msg.ui_action?.type === 'navigate' && msg.ui_action.path) {
            navigate(msg.ui_action.path);
          }
        });

        if (data.conversation_state) {
          setConversationState(data.conversation_state);
        }
      } catch (err) {
        console.error('Failed to parse SSE data:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
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
        { method: 'POST', body: JSON.stringify(buildChatRequest(message, model, dagContexts)) }
      );
      setConversationId(data.conversation_id);
      return data.conversation_id;
    },
    [baseUrl, setConversationId]
  );

  const sendMessage = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<void> => {
      setIsSending(true);
      setError(null);

      // Optimistic update - show user message immediately
      const optimisticMessage = {
        id: `optimistic-${Date.now()}`,
        conversation_id: conversationId || '',
        type: 'user' as const,
        sequence_id: Date.now(),
        content: message,
        created_at: new Date().toISOString(),
      };
      addMessage(optimisticMessage);

      try {
        if (!conversationId) {
          await startConversation(message, model, dagContexts);
          return;
        }
        await fetchWithAuth(`${baseUrl}/conversations/${conversationId}/chat`, {
          method: 'POST',
          body: JSON.stringify(buildChatRequest(message, model, dagContexts)),
        });
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to send message';
        setError(errorMessage);
        throw err;
      } finally {
        setIsSending(false);
      }
    },
    [baseUrl, conversationId, startConversation, addMessage]
  );

  const cancelConversation = useCallback(async (): Promise<void> => {
    if (!conversationId) return;
    await fetchWithAuth(`${baseUrl}/conversations/${conversationId}/cancel`, { method: 'POST' });
  }, [baseUrl, conversationId]);

  const fetchConversations = useCallback(async (): Promise<void> => {
    try {
      const data = await fetchWithAuth<ConversationWithState[]>(`${baseUrl}/conversations`);
      setConversations(data || []);
    } catch (err) {
      console.error('Failed to fetch conversations:', err);
      setConversations([]);
    }
  }, [baseUrl, setConversations]);

  const selectConversation = useCallback(
    async (id: string): Promise<void> => {
      const data = await fetchWithAuth<StreamResponse>(`${baseUrl}/conversations/${id}`);
      setConversationId(id);
      setMessages(data.messages || []);
      if (data.conversation_state) {
        setConversationState(data.conversation_state);
      }
    },
    [baseUrl, setConversationId, setMessages, setConversationState]
  );

  const isWorking = isSending || conversationState?.working === true;

  const clearError = useCallback(() => setError(null), []);

  return {
    conversationId,
    messages,
    conversationState,
    conversations,
    isWorking,
    error,
    startConversation,
    sendMessage,
    cancelConversation,
    clearConversation,
    clearError,
    fetchConversations,
    selectConversation,
  };
}
