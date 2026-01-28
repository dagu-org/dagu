import {
  createContext,
  useCallback,
  useContext,
  useState,
} from 'react';
import type { Dispatch, ReactNode, SetStateAction } from 'react';
import type {
  ConversationState,
  ConversationWithState,
  Message,
} from '../types';

interface AgentChatContextType {
  isOpen: boolean;
  conversationId: string | null;
  messages: Message[];
  conversationState: ConversationState | null;
  conversations: ConversationWithState[];
  openChat: () => void;
  closeChat: () => void;
  toggleChat: () => void;
  setConversationId: (id: string | null) => void;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setConversationState: (state: ConversationState | null) => void;
  setConversations: (conversations: ConversationWithState[]) => void;
  addMessage: (message: Message) => void;
  clearConversation: () => void;
}

interface AgentChatProviderProps {
  children: ReactNode;
}

const AgentChatContext = createContext<AgentChatContextType | null>(null);

export function AgentChatProvider({ children }: AgentChatProviderProps): ReactNode {
  const [isOpen, setIsOpen] = useState(false);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversationState, setConversationState] =
    useState<ConversationState | null>(null);
  const [conversations, setConversations] = useState<ConversationWithState[]>(
    []
  );

  const openChat = useCallback(() => setIsOpen(true), []);
  const closeChat = useCallback(() => setIsOpen(false), []);
  const toggleChat = useCallback(() => setIsOpen((prev) => !prev), []);

  const addMessage = useCallback((message: Message) => {
    setMessages((prev) => {
      const existingIndex = prev.findIndex((m) => m.id === message.id);
      if (existingIndex === -1) {
        return [...prev, message];
      }
      const updated = [...prev];
      updated[existingIndex] = message;
      return updated;
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
        conversations,
        openChat,
        closeChat,
        toggleChat,
        setConversationId,
        setMessages,
        setConversationState,
        setConversations,
        addMessage,
        clearConversation,
      }}
    >
      {children}
    </AgentChatContext.Provider>
  );
}

export function useAgentChatContext(): AgentChatContextType {
  const context = useContext(AgentChatContext);
  if (!context) {
    throw new Error(
      'useAgentChatContext must be used within an AgentChatProvider'
    );
  }
  return context;
}
