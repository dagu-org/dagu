import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
} from 'react';
import type { Dispatch, ReactNode, SetStateAction } from 'react';
import type {
  SessionState,
  SessionWithState,
  Message,
} from '../types';

interface AgentChatContextType {
  isOpen: boolean;
  isClosing: boolean;
  sessionId: string | null;
  messages: Message[];
  pendingUserMessage: string | null;
  sessionState: SessionState | null;
  sessions: SessionWithState[];
  openChat: () => void;
  closeChat: () => void;
  toggleChat: () => void;
  setSessionId: (id: string | null) => void;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setSessionState: (state: SessionState | null) => void;
  setSessions: (sessions: SessionWithState[]) => void;
  addMessage: (message: Message) => void;
  setPendingUserMessage: (message: string | null) => void;
  clearSession: () => void;
}

interface AgentChatProviderProps {
  children: ReactNode;
}

const AgentChatContext = createContext<AgentChatContextType | null>(null);

export function AgentChatProvider({ children }: AgentChatProviderProps): ReactNode {
  const [isOpen, setIsOpen] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [pendingUserMessage, setPendingUserMessage] = useState<string | null>(null);
  const [sessionState, setSessionState] =
    useState<SessionState | null>(null);
  const [sessions, setSessions] = useState<SessionWithState[]>(
    []
  );

  const openChat = useCallback(() => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
    setIsClosing(false);
    setIsOpen(true);
  }, []);

  const closeChat = useCallback(() => {
    if (!isOpen || isClosing) return;
    setIsClosing(true);
    closeTimerRef.current = setTimeout(() => {
      setIsClosing(false);
      setIsOpen(false);
      closeTimerRef.current = null;
    }, 250);
  }, [isOpen, isClosing]);

  const toggleChat = useCallback(() => {
    if (isOpen) {
      closeChat();
    } else {
      openChat();
    }
  }, [isOpen, closeChat, openChat]);

  const addMessage = useCallback((message: Message) => {
    // When a user message arrives from SSE, clear the pending message
    if (message.type === 'user') {
      setPendingUserMessage(null);
    }

    setMessages((prev) => {
      // Check for exact ID match - update existing
      const existingIndex = prev.findIndex((m) => m.id === message.id);
      if (existingIndex !== -1) {
        const updated = [...prev];
        updated[existingIndex] = message;
        return updated;
      }

      // Add as new message
      return [...prev, message];
    });
  }, []);

  const clearSession = useCallback(() => {
    setSessionId(null);
    setMessages([]);
    setPendingUserMessage(null);
    setSessionState(null);
  }, []);

  return (
    <AgentChatContext.Provider
      value={{
        isOpen,
        isClosing,
        sessionId,
        messages,
        pendingUserMessage,
        sessionState,
        sessions,
        openChat,
        closeChat,
        toggleChat,
        setSessionId,
        setMessages,
        setSessionState,
        setSessions,
        addMessage,
        setPendingUserMessage,
        clearSession,
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
