import { ReactElement, useCallback, useEffect, useRef } from 'react';

import { AlertCircle, Plus, Shield, ShieldOff, Terminal, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';

import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
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
  const { preferences, updatePreference } = useUserPreferences();
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
    (message: string, dagContexts?: DAGContext[]): void => {
      sendMessage(message, undefined, dagContexts).catch(() => {});
    },
    [sendMessage]
  );

  const handleCancel = useCallback((): void => {
    cancelConversation().catch((err) =>
      setError(err instanceof Error ? err.message : 'Failed to cancel')
    );
  }, [cancelConversation]);

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
    [selectConversation, clearConversation]
  );

  if (!isOpen) return null;

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
      {/* Resize handles */}
      <div
        className="absolute top-0 left-2 right-2 h-1.5 cursor-n-resize"
        {...resizeHandlers.top}
      />
      <div
        className="absolute bottom-0 left-2 right-2 h-1.5 cursor-s-resize"
        {...resizeHandlers.bottom}
      />
      <div
        className="absolute left-0 top-2 bottom-2 w-1.5 cursor-w-resize"
        {...resizeHandlers.left}
      />
      <div
        className="absolute right-0 top-2 bottom-2 w-1.5 cursor-e-resize"
        {...resizeHandlers.right}
      />
      <div
        className="absolute top-0 left-0 w-3 h-3 cursor-nw-resize"
        {...resizeHandlers.topLeft}
      />
      <div
        className="absolute top-0 right-0 w-3 h-3 cursor-ne-resize"
        {...resizeHandlers.topRight}
      />
      <div
        className="absolute bottom-0 left-0 w-3 h-3 cursor-sw-resize"
        {...resizeHandlers.bottomLeft}
      />
      <div
        className="absolute bottom-0 right-0 w-3 h-3 cursor-se-resize"
        {...resizeHandlers.bottomRight}
      />

      <div
        className="flex items-center justify-between px-3 py-2 border-b border-border bg-secondary dark:bg-surface cursor-move"
        {...dragHandlers}
      >
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          <Select
            value={conversationId || 'new'}
            onValueChange={handleSelectConversation}
          >
            <SelectTrigger className="h-6 w-auto max-w-[200px] px-2 text-xs bg-transparent border-none shadow-none hover:bg-accent">
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
                  {formatDate(conv.conversation.created_at)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <TooltipProvider delayDuration={300}>
          <div className="flex items-center gap-1 flex-shrink-0">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => updatePreference('safeMode', !preferences.safeMode)}
                  className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                >
                  {preferences.safeMode ? (
                    <Shield className="h-4 w-4" />
                  ) : (
                    <ShieldOff className="h-4 w-4" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>
                  {preferences.safeMode
                    ? "Safe mode enabled: dangerous commands require approval"
                    : "Safe mode disabled: all commands execute immediately"}
                </p>
              </TooltipContent>
            </Tooltip>
            <Button
              variant="ghost"
              size="sm"
              onClick={clearConversation}
              className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
              title="New conversation"
            >
              <Plus className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={closeChat}
              className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
              title="Close"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </TooltipProvider>
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
    </div>
  );
}
