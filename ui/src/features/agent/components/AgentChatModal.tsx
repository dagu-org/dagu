import { ReactElement, useCallback, useEffect, useRef } from 'react';

import { AlertCircle, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { useIsMobile } from '@/hooks/useIsMobile';

import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { ConversationWithState, DAGContext } from '../types';
import { AgentChatModalHeader } from './AgentChatModalHeader';
import { ChatInput } from './ChatInput';
import { ChatMessages } from './ChatMessages';
import { ResizeHandles } from './ResizeHandles';

function findLatestConversation(
  conversations: ConversationWithState[]
): ConversationWithState | null {
  if (conversations.length === 0) return null;

  let latest = conversations[0]!;
  for (const conv of conversations) {
    if (
      new Date(conv.conversation.updated_at) >
      new Date(latest.conversation.updated_at)
    ) {
      latest = conv;
    }
  }
  return latest;
}

export function AgentChatModal(): ReactElement | null {
  const { isOpen, closeChat } = useAgentChatContext();
  const isMobile = useIsMobile();
  const {
    conversationId,
    messages,
    pendingUserMessage,
    conversations,
    isWorking,
    error,
    answeredPrompts,
    setError,
    sendMessage,
    cancelConversation,
    clearConversation,
    clearError,
    fetchConversations,
    selectConversation,
    respondToPrompt,
  } = useAgentChat();
  const { bounds, dragHandlers, resizeHandlers } = useResizableDraggable();

  const hasAutoSelectedRef = useRef(false);

  useEffect(() => {
    if (isOpen) {
      hasAutoSelectedRef.current = false;
      fetchConversations();
    }
  }, [isOpen, fetchConversations]);

  useEffect(() => {
    if (
      isOpen &&
      conversations.length > 0 &&
      !conversationId &&
      !hasAutoSelectedRef.current
    ) {
      hasAutoSelectedRef.current = true;
      const latest = findLatestConversation(conversations);
      if (latest) {
        selectConversation(latest.conversation.id).catch(() => {});
      }
    }
  }, [isOpen, conversations, conversationId, selectConversation]);

  const handleSend = useCallback(
    (message: string, dagContexts?: DAGContext[], model?: string): void => {
      sendMessage(message, model, dagContexts).catch(() => {});
    },
    [sendMessage]
  );

  const handleCancel = useCallback((): void => {
    cancelConversation().catch((err) =>
      setError(err instanceof Error ? err.message : 'Failed to cancel')
    );
  }, [cancelConversation, setError]);

  const handleSelectConversation = useCallback(
    (value: string): void => {
      if (value === 'new') {
        clearConversation();
        return;
      }
      selectConversation(value).catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to select conversation')
      );
    },
    [selectConversation, clearConversation, setError]
  );

  if (!isOpen) return null;

  const errorBanner = error && (
    <div className="mx-3 mt-2 p-2 bg-destructive/10 border border-destructive/20 rounded-md flex items-start gap-2">
      <AlertCircle className="h-4 w-4 text-destructive flex-shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <p className="text-xs text-destructive">{error}</p>
      </div>
      <button
        onClick={clearError}
        className="text-destructive/60 hover:text-destructive flex-shrink-0"
      >
        <X className="h-3 w-3" />
      </button>
    </div>
  );

  const content = (
    <>
      <AgentChatModalHeader
        conversationId={conversationId}
        conversations={conversations}
        onSelectConversation={handleSelectConversation}
        onClearConversation={clearConversation}
        onClose={closeChat}
        dragHandlers={isMobile ? undefined : dragHandlers}
        isMobile={isMobile}
      />
      {errorBanner}
      <ChatMessages
        messages={messages}
        pendingUserMessage={pendingUserMessage}
        isWorking={isWorking}
        onPromptRespond={respondToPrompt}
        answeredPrompts={answeredPrompts}
      />
      <ChatInput
        onSend={handleSend}
        onCancel={handleCancel}
        isWorking={isWorking}
        placeholder="Ask me to create a DAG, run a command..."
      />
    </>
  );

  // Mobile: fullscreen
  if (isMobile) {
    return (
      <div
        className={cn(
          'fixed inset-0 z-50',
          'flex flex-col',
          'bg-popover dark:bg-zinc-950',
          'animate-in slide-in-from-bottom-4 fade-in-0 duration-200'
        )}
      >
        {content}
      </div>
    );
  }

  // Desktop: resizable/draggable window
  return (
    <div
      className={cn(
        'fixed z-50',
        'flex flex-col',
        'bg-popover dark:bg-zinc-950 border border-border rounded-lg overflow-hidden',
        'shadow-xl dark:shadow-[0_0_30px_rgba(0,0,0,0.6)]',
        'animate-in slide-in-from-bottom-4 fade-in-0 duration-200'
      )}
      style={{
        right: bounds.right,
        bottom: bounds.bottom,
        width: bounds.width,
        height: bounds.height,
        maxWidth: 'calc(100vw - 32px)',
        maxHeight: 'calc(100vh - 100px)',
      }}
    >
      <ResizeHandles resizeHandlers={resizeHandlers} />
      {content}
    </div>
  );
}
