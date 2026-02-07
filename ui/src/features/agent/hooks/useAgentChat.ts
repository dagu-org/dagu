import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfig } from '@/contexts/ConfigContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { useNamespace } from '@/contexts/NamespaceContext';
import { AppBarContext } from '@/contexts/AppBarContext';
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
  safeMode?: boolean,
  namespace?: string
): ChatRequest {
  return { message, model, dag_contexts: dagContexts, safe_mode: safeMode, namespace };
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

function buildStreamUrl(baseUrl: string, conversationId: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}/conversations/${conversationId}/stream`, window.location.origin);
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
  const { selectedNamespace, isAllNamespaces } = useNamespace();
  const appBarContext = useContext(AppBarContext);
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
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  // Agent conversations are scoped to a namespace. When "All Namespaces" is
  // selected, default to "default" since agents cannot operate across namespaces.
  const agentNamespace = isAllNamespaces ? 'default' : selectedNamespace;

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

    const eventSource = new EventSource(buildStreamUrl(baseUrl, conversationId, remoteNode));
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
  }, [conversationId, baseUrl, remoteNode, addMessage, setConversationState, setConversationId, navigate, closeEventSource]);

  const startConversation = useCallback(
    async (message: string, model?: string, dagContexts?: DAGContext[]): Promise<string> => {
      const data = await fetchWithAuth<NewConversationResponse>(
        buildApiUrl(baseUrl, '/conversations/new', remoteNode),
        { method: 'POST', body: JSON.stringify(buildChatRequest(message, model, dagContexts, preferences.safeMode, agentNamespace)) }
      );
      setConversationId(data.conversation_id);
      // Refresh conversations list to include the new one
      const convs = await fetchWithAuth<ConversationWithState[]>(buildApiUrl(baseUrl, '/conversations', remoteNode));
      setConversations(convs || []);
      return data.conversation_id;
    },
    [baseUrl, remoteNode, setConversationId, setConversations, preferences.safeMode, agentNamespace]
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
        await fetchWithAuth(buildApiUrl(baseUrl, `/conversations/${conversationId}/chat`, remoteNode), {
          method: 'POST',
          body: JSON.stringify(buildChatRequest(message, model, dagContexts, preferences.safeMode, agentNamespace)),
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to send message');
        setPendingUserMessage(null);
        throw err;
      } finally {
        setIsSending(false);
      }
    },
    [baseUrl, remoteNode, conversationId, startConversation, setPendingUserMessage, preferences.safeMode, agentNamespace]
  );

  const cancelConversation = useCallback(async (): Promise<void> => {
    if (!conversationId) return;
    await fetchWithAuth(buildApiUrl(baseUrl, `/conversations/${conversationId}/cancel`, remoteNode), { method: 'POST' });
  }, [baseUrl, remoteNode, conversationId]);

  const respondToPrompt = useCallback(async (response: UserPromptResponse, displayValue: string): Promise<void> => {
    if (!conversationId) return;

    try {
      await fetchWithAuth(buildApiUrl(baseUrl, `/conversations/${conversationId}/respond`, remoteNode), {
        method: 'POST',
        body: JSON.stringify(response),
      });
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
    } catch (err) {
      // If prompt not found (e.g., after reload), mark as answered anyway to update UI
      setAnsweredPrompts(prev => ({ ...prev, [response.prompt_id]: displayValue }));
      setError(err instanceof Error ? err.message : 'Failed to submit response');
    }
  }, [baseUrl, remoteNode, conversationId]);

  const fetchConversations = useCallback(async (): Promise<void> => {
    try {
      const data = await fetchWithAuth<ConversationWithState[]>(buildApiUrl(baseUrl, '/conversations', remoteNode));
      setConversations(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch conversations');
      setConversations([]);
    }
  }, [baseUrl, remoteNode, setConversations]);

  const selectConversation = useCallback(
    async (id: string): Promise<void> => {
      const data = await fetchWithAuth<StreamResponse>(buildApiUrl(baseUrl, `/conversations/${id}`, remoteNode));
      setConversationId(id);
      setMessages(data.messages || []);
      setAnsweredPrompts({}); // Clear answered prompts when switching conversations
      if (data.conversation_state) {
        setConversationState(data.conversation_state);
      }
    },
    [baseUrl, remoteNode, setConversationId, setMessages, setConversationState]
  );

  const isWorking = isSending || !!conversationState?.working;

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
