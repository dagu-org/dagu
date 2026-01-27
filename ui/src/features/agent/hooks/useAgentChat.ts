import { useCallback, useEffect, useRef } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { useAgentChatContext } from '../context/AgentChatContext';
import {
  StreamResponse,
  NewConversationResponse,
  ChatRequest,
  Message,
} from '../types';

export function useAgentChat() {
  const config = useConfig();
  const {
    conversationId,
    messages,
    conversationState,
    setConversationId,
    setMessages,
    setConversationState,
    addMessage,
    clearConversation,
  } = useAgentChatContext();

  const eventSourceRef = useRef<EventSource | null>(null);
  const baseUrl = `${config.basePath}/api/v2/agent`;

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

    // Create new SSE connection
    const streamUrl = `${baseUrl}/conversations/${conversationId}/stream`;
    const eventSource = new EventSource(streamUrl);
    eventSourceRef.current = eventSource;

    eventSource.onmessage = (event) => {
      try {
        const data: StreamResponse = JSON.parse(event.data);

        // Update messages
        if (data.messages) {
          data.messages.forEach((msg: Message) => {
            addMessage(msg);
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
  }, [conversationId, baseUrl, addMessage, setConversationState, setConversationId]);

  // Start a new conversation
  const startConversation = useCallback(
    async (message: string, model?: string): Promise<string> => {
      const response = await fetch(`${baseUrl}/conversations/new`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
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
      if (!conversationId) {
        await startConversation(message, model);
        return;
      }

      const response = await fetch(
        `${baseUrl}/conversations/${conversationId}/chat`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            message,
            model,
          } as ChatRequest),
        }
      );

      if (!response.ok) {
        throw new Error(`Failed to send message: ${response.statusText}`);
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
      }
    );

    if (!response.ok) {
      throw new Error(`Failed to cancel conversation: ${response.statusText}`);
    }
  }, [baseUrl, conversationId]);

  return {
    conversationId,
    messages,
    conversationState,
    isWorking: conversationState?.working ?? false,
    startConversation,
    sendMessage,
    cancelConversation,
    clearConversation,
  };
}
