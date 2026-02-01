import { HTMLAttributes, ReactElement, useCallback, useEffect, useRef } from 'react';

import { AlertCircle, Plus, Shield, ShieldOff, Terminal, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';

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

export interface AgentChatContentProps {
  /** Affects spacing/styling - 'modal' is compact, 'page' is spacious */
  variant?: 'modal' | 'page';
  /** Additional container classes */
  className?: string;
  /** Whether to show the close button */
  showCloseButton?: boolean;
  /** Close button handler */
  onClose?: () => void;
  /** Props to spread on the header (e.g., for drag handlers in modal) */
  headerProps?: HTMLAttributes<HTMLDivElement>;
  /** Auto-select latest conversation on mount (default: true) */
  autoSelectLatest?: boolean;
}

export function AgentChatContent({
  variant = 'modal',
  className,
  showCloseButton = false,
  onClose,
  headerProps,
  autoSelectLatest = true,
}: AgentChatContentProps): ReactElement {
  const { isOpen } = useAgentChatContext();
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

  const hasAutoSelectedRef = useRef(false);

  useEffect(() => {
    if (isOpen) {
      hasAutoSelectedRef.current = false;
      fetchConversations();
    }
  }, [isOpen, fetchConversations]);

  useEffect(() => {
    if (
      autoSelectLatest &&
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
  }, [autoSelectLatest, isOpen, conversations, conversationId, selectConversation]);

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

  const isCompact = variant === 'modal';
  const headerPadding = isCompact ? 'px-3 py-2' : 'px-4 py-3';
  const errorMargin = isCompact ? 'mx-3 mt-2' : 'mx-4 mt-3';

  return (
    <div className={cn('flex flex-col flex-1 min-h-0', className)}>
      <div
        className={cn(
          'flex items-center justify-between border-b border-border bg-secondary dark:bg-surface',
          headerPadding,
          headerProps?.className
        )}
        {...headerProps}
      >
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          <Select
            value={conversationId || 'new'}
            onValueChange={handleSelectConversation}
          >
            <SelectTrigger className="h-6 w-auto px-2 text-xs bg-transparent border-none shadow-none hover:bg-accent">
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
                    <span>
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
        </div>
        <div className="flex items-center gap-1 flex-shrink-0">
          <div
            className="flex items-center gap-1 px-1"
            title={preferences.safeMode
              ? "Safe Mode ON: Dangerous commands require approval"
              : "Safe Mode OFF: All commands execute immediately"}
          >
            <Switch
              checked={preferences.safeMode}
              onCheckedChange={(checked) => updatePreference('safeMode', checked)}
              className="h-4 w-7 data-[state=checked]:bg-green-600"
            />
            {preferences.safeMode ? (
              <Shield className="h-3.5 w-3.5 text-green-500" />
            ) : (
              <ShieldOff className="h-3.5 w-3.5 text-muted-foreground" />
            )}
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={clearConversation}
            className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
            title="New conversation"
          >
            <Plus className="h-4 w-4" />
          </Button>
          {showCloseButton && onClose && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
              className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
              title="Close"
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>

      {error && (
        <div className={cn('p-2 bg-destructive/10 border border-destructive/20 rounded-md flex items-start gap-2', errorMargin)}>
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
