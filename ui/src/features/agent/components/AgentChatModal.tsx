import { ReactElement, useCallback, useEffect, useRef } from 'react';

import { AlertCircle, Plus, Terminal, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { ConversationWithState, DAGContext } from '../types';
import { ChatInput } from './ChatInput';
import { ChatMessages } from './ChatMessages';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

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
  const {
    conversationId,
    messages,
    conversations,
    isWorking,
    error,
    sendMessage,
    cancelConversation,
    clearConversation,
    clearError,
    fetchConversations,
    selectConversation,
  } = useAgentChat();

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
        selectConversation(latest.conversation.id).catch(console.error);
      }
    }
  }, [isOpen, conversations, conversationId, selectConversation]);

  const handleSend = useCallback(
    (message: string, dagContexts?: DAGContext[]): void => {
      sendMessage(message, undefined, dagContexts).catch((err) =>
        console.error('Failed to send message:', err)
      );
    },
    [sendMessage]
  );

  const handleCancel = useCallback((): void => {
    cancelConversation().catch((err) =>
      console.error('Failed to cancel:', err)
    );
  }, [cancelConversation]);

  const handleSelectConversation = useCallback(
    (value: string): void => {
      if (value === 'new') {
        clearConversation();
        return;
      }
      selectConversation(value).catch((err) =>
        console.error('Failed to select conversation:', err)
      );
    },
    [selectConversation, clearConversation]
  );

  if (!isOpen) return null;

  return (
    <div
      className={cn(
        'fixed bottom-16 right-4 z-50',
        'w-[440px] max-w-[calc(100vw-32px)]',
        'h-[540px] max-h-[calc(100vh-100px)]',
        'flex flex-col',
        'bg-popover dark:bg-zinc-950 border border-border rounded-lg overflow-hidden',
        'shadow-xl dark:shadow-[0_0_30px_rgba(0,0,0,0.6)]',
        'animate-in slide-in-from-bottom-4 fade-in-0 duration-200'
      )}
    >
      <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-muted/80">
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          <Select
            value={conversationId || 'new'}
            onValueChange={handleSelectConversation}
          >
            <SelectTrigger className="h-6 w-auto max-w-[200px] px-2 text-xs bg-transparent border-border hover:bg-muted">
              <SelectValue placeholder="New conversation" />
            </SelectTrigger>
            <SelectContent className="bg-popover border-border">
              <SelectItem value="new" className="text-xs">
                <div className="flex items-center gap-1.5">
                  <Plus className="h-3 w-3" />
                  New conversation
                </div>
              </SelectItem>
              {conversations.map((conv) => (
                <SelectItem
                  key={conv.conversation.id}
                  value={conv.conversation.id}
                  className="text-xs"
                >
                  <div className="flex items-center gap-1.5">
                    <span className="truncate">
                      {formatDate(conv.conversation.created_at)}
                    </span>
                    {conv.working && (
                      <span className="text-yellow-500 text-[10px]">...</span>
                    )}
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {isWorking && (
            <span className="text-xs text-yellow-500 font-mono flex-shrink-0">
              running...
            </span>
          )}
        </div>
        <div className="flex items-center gap-0.5 flex-shrink-0">
          <Button
            variant="ghost"
            size="sm"
            onClick={clearConversation}
            className="h-6 w-6 p-0 text-muted-foreground hover:text-foreground"
            title="New conversation"
          >
            <Plus className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={closeChat}
            className="h-6 w-6 p-0 text-muted-foreground hover:text-foreground"
            title="Close"
          >
            <X className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {error && (
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
      )}

      <ChatMessages messages={messages} isWorking={isWorking} />

      <ChatInput
        onSend={handleSend}
        onCancel={handleCancel}
        isWorking={isWorking}
        placeholder="Ask me to create a DAG, run a command..."
      />
    </div>
  );
}
