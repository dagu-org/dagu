import * as React from 'react';
import { createContext, useContext, useState, useCallback } from 'react';
import { Message, ConversationState } from '../types';

interface AgentChatContextType {
  isOpen: boolean;
  conversationId: string | null;
  messages: Message[];
  conversationState: ConversationState | null;
  openChat: () => void;
  closeChat: () => void;
  toggleChat: () => void;
  setConversationId: (id: string | null) => void;
  setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
  setConversationState: (state: ConversationState | null) => void;
  addMessage: (message: Message) => void;
  clearConversation: () => void;
}

const AgentChatContext = createContext<AgentChatContextType | null>(null);

export function AgentChatProvider({ children }: { children: React.ReactNode }) {
  const [isOpen, setIsOpen] = useState(false);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversationState, setConversationState] =
    useState<ConversationState | null>(null);

  const openChat = useCallback(() => setIsOpen(true), []);
  const closeChat = useCallback(() => setIsOpen(false), []);
  const toggleChat = useCallback(() => setIsOpen((prev) => !prev), []);

  const addMessage = useCallback((message: Message) => {
    setMessages((prev) => {
      // Check if message already exists (by id)
      const exists = prev.some((m) => m.id === message.id);
      if (exists) {
        // Update existing message
        return prev.map((m) => (m.id === message.id ? message : m));
      }
      // Add new message
      return [...prev, message];
    });
  }, []);

  const clearConversation = useCallback(() => {
    setConversationId(null);
    setMessages([]);
    setConversationState(null);
  }, []);

  return (
    <AgentChatContext.Provider
      value={{
        isOpen,
        conversationId,
        messages,
        conversationState,
        openChat,
        closeChat,
        toggleChat,
        setConversationId,
        setMessages,
        setConversationState,
        addMessage,
        clearConversation,
      }}
    >
      {children}
    </AgentChatContext.Provider>
  );
}

export function useAgentChatContext() {
  const context = useContext(AgentChatContext);
  if (!context) {
    throw new Error(
      'useAgentChatContext must be used within an AgentChatProvider'
    );
  }
  return context;
}
