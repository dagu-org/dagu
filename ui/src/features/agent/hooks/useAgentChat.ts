import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfig } from '@/contexts/ConfigContext';
import { useAgentChatContext } from '../context/AgentChatContext';
import {
  StreamResponse,
  NewConversationResponse,
  ChatRequest,
  Message,
  ConversationWithState,
} from '../types';

const TOKEN_KEY = 'dagu_auth_token';

// Helper to get auth headers
function getAuthHeaders(): HeadersInit {
  const token = localStorage.getItem(TOKEN_KEY);
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };
  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

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
  const [isSending, setIsSending] = useState(false);
  const baseUrl = `${config.apiURL}/agent`;

  // Clean up EventSource on unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, []);

  // Subscribe to SSE when conversation ID changes
  useEffect(() => {
    if (!conversationId) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      return;
    }

    // Close existing connection
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    // Create new SSE connection with auth token as query param
    // (EventSource doesn't support custom headers)
    const token = localStorage.getItem(TOKEN_KEY);
    const streamUrl = new URL(`${baseUrl}/conversations/${conversationId}/stream`, window.location.origin);
    if (token) {
      streamUrl.searchParams.set('token', token);
    }
    const eventSource = new EventSource(streamUrl.toString());
    eventSourceRef.current = eventSource;

    eventSource.onmessage = (event) => {
      try {
        const data: StreamResponse = JSON.parse(event.data);

        // Update messages
        if (data.messages) {
          data.messages.forEach((msg: Message) => {
            addMessage(msg);

            // Handle UI actions
            if (msg.type === 'ui_action' && msg.ui_action) {
              if (msg.ui_action.type === 'navigate' && msg.ui_action.path) {
                navigate(msg.ui_action.path);
              }
            }
          });
        }

        // Update conversation state
        if (data.conversation_state) {
          setConversationState(data.conversation_state);
        }
      } catch (err) {
        console.error('Failed to parse SSE data:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
      // Reconnect after a delay if the connection was closed
      if (eventSource.readyState === EventSource.CLOSED) {
        setTimeout(() => {
          if (conversationId && eventSourceRef.current === eventSource) {
            // Trigger re-subscription
            setConversationId(conversationId);
          }
        }, 1000);
      }
    };

    return () => {
      eventSource.close();
    };
  }, [conversationId, baseUrl, addMessage, setConversationState, setConversationId, navigate]);

  // Start a new conversation
  const startConversation = useCallback(
    async (message: string, model?: string): Promise<string> => {
      const response = await fetch(`${baseUrl}/conversations/new`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({
          message,
          model,
        } as ChatRequest),
      });

      if (!response.ok) {
        throw new Error(`Failed to start conversation: ${response.statusText}`);
      }

      const data: NewConversationResponse = await response.json();
      setConversationId(data.conversation_id);
      return data.conversation_id;
    },
    [baseUrl, setConversationId]
  );

  // Send a message to existing conversation
  const sendMessage = useCallback(
    async (message: string, model?: string): Promise<void> => {
      setIsSending(true);
      try {
        if (!conversationId) {
          await startConversation(message, model);
          return;
        }

        const response = await fetch(
          `${baseUrl}/conversations/${conversationId}/chat`,
          {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({
              message,
              model,
            } as ChatRequest),
          }
        );

        if (!response.ok) {
          throw new Error(`Failed to send message: ${response.statusText}`);
        }
      } finally {
        setIsSending(false);
      }
    },
    [baseUrl, conversationId, startConversation]
  );

  // Cancel the current conversation
  const cancelConversation = useCallback(async (): Promise<void> => {
    if (!conversationId) return;

    const response = await fetch(
      `${baseUrl}/conversations/${conversationId}/cancel`,
      {
        method: 'POST',
        headers: getAuthHeaders(),
      }
    );

    if (!response.ok) {
      throw new Error(`Failed to cancel conversation: ${response.statusText}`);
    }
  }, [baseUrl, conversationId]);

  // Fetch all conversations for the current user
  const fetchConversations = useCallback(async (): Promise<void> => {
    try {
      const response = await fetch(`${baseUrl}/conversations`, {
        headers: getAuthHeaders(),
      });
      if (!response.ok) {
        throw new Error(`Failed to fetch conversations: ${response.statusText}`);
      }
      const data: ConversationWithState[] = await response.json();
      setConversations(data || []);
    } catch (err) {
      console.error('Failed to fetch conversations:', err);
      setConversations([]);
    }
  }, [baseUrl, setConversations]);

  // Select and load an existing conversation
  const selectConversation = useCallback(
    async (id: string): Promise<void> => {
      try {
        const response = await fetch(`${baseUrl}/conversations/${id}`, {
          headers: getAuthHeaders(),
        });
        if (!response.ok) {
          throw new Error(`Failed to load conversation: ${response.statusText}`);
        }
        const data: StreamResponse = await response.json();

        // Update state with the loaded conversation
        setConversationId(id);
        setMessages(data.messages || []);
        if (data.conversation_state) {
          setConversationState(data.conversation_state);
        }
      } catch (err) {
        console.error('Failed to select conversation:', err);
        throw err;
      }
    },
    [baseUrl, setConversationId, setMessages, setConversationState]
  );

  return {
    conversationId,
    messages,
    conversationState,
    conversations,
    isWorking: isSending || (conversationState?.working ?? false),
    startConversation,
    sendMessage,
    cancelConversation,
    clearConversation,
    fetchConversations,
    selectConversation,
  };
}
