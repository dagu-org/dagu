import * as React from 'react';
import { X, Terminal, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { ChatMessages } from './ChatMessages';
import { ChatInput } from './ChatInput';
import { cn } from '@/lib/utils';
import { DAGContext } from '../types';

export function AgentChatModal() {
  const { isOpen, closeChat } = useAgentChatContext();
  const {
    conversationId,
    messages,
    conversations,
    isWorking,
    sendMessage,
    cancelConversation,
    clearConversation,
    fetchConversations,
    selectConversation,
  } = useAgentChat();

  const hasAutoSelectedRef = React.useRef(false);

  // Fetch conversations when modal opens
  React.useEffect(() => {
    if (isOpen) {
      hasAutoSelectedRef.current = false;
      fetchConversations();
    }
  }, [isOpen, fetchConversations]);

  // Auto-select latest conversation when loaded
  React.useEffect(() => {
    if (isOpen && conversations.length > 0 && !conversationId && !hasAutoSelectedRef.current) {
      hasAutoSelectedRef.current = true;
      // Find the latest conversation by updated_at
      const latest = conversations.reduce((a, b) =>
        new Date(a.conversation.updated_at) > new Date(b.conversation.updated_at) ? a : b
      );
      selectConversation(latest.conversation.id).catch(console.error);
    }
  }, [isOpen, conversations, conversationId, selectConversation]);

  const handleSend = React.useCallback(
    async (message: string, dagContexts?: DAGContext[]) => {
      try {
        await sendMessage(message, undefined, dagContexts);
      } catch (err) {
        console.error('Failed to send message:', err);
      }
    },
    [sendMessage]
  );

  const handleCancel = React.useCallback(async () => {
    try {
      await cancelConversation();
    } catch (err) {
      console.error('Failed to cancel:', err);
    }
  }, [cancelConversation]);

  const handleClear = React.useCallback(() => {
    clearConversation();
  }, [clearConversation]);

  const handleSelectConversation = React.useCallback(
    async (value: string) => {
      if (value === 'new') {
        clearConversation();
      } else {
        try {
          await selectConversation(value);
        } catch (err) {
          console.error('Failed to select conversation:', err);
        }
      }
    },
    [selectConversation, clearConversation]
  );

  // Format date for display
  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  if (!isOpen) return null;

  return (
    <div
      className={cn(
        'fixed bottom-16 right-4 z-50',
        'w-[440px] max-w-[calc(100vw-32px)]',
        'h-[540px] max-h-[calc(100vh-100px)]',
        'flex flex-col',
        'bg-zinc-950 border-2 border-zinc-600 rounded-lg overflow-hidden',
        'shadow-[0_0_40px_rgba(0,0,0,0.8)]',
        'animate-in slide-in-from-bottom-4 fade-in-0 duration-200'
      )}
    >
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-800 bg-zinc-900/80">
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            <Select
              value={conversationId || 'new'}
              onValueChange={handleSelectConversation}
            >
              <SelectTrigger className="h-6 w-auto max-w-[200px] px-2 text-xs bg-transparent border-zinc-700 hover:bg-zinc-800">
                <SelectValue placeholder="New conversation" />
              </SelectTrigger>
              <SelectContent className="bg-zinc-900 border-zinc-700">
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
              <span className="text-xs text-yellow-500 font-mono flex-shrink-0">running...</span>
            )}
          </div>
          <div className="flex items-center gap-0.5 flex-shrink-0">
            <Button
              variant="ghost"
              size="sm"
              onClick={handleClear}
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

      {/* Messages */}
      <ChatMessages messages={messages} isWorking={isWorking} />

      {/* Input */}
      <ChatInput
        onSend={handleSend}
        onCancel={handleCancel}
        isWorking={isWorking}
        placeholder="Ask me to create a DAG, run a command..."
      />
    </div>
  );
}
