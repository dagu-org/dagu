import * as React from 'react';
import { X, Trash2, Minimize2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { ChatMessages } from './ChatMessages';
import { ChatInput } from './ChatInput';
import { cn } from '@/lib/utils';

export function AgentChatModal() {
  const { isOpen, closeChat } = useAgentChatContext();
  const {
    messages,
    isWorking,
    sendMessage,
    cancelConversation,
    clearConversation,
  } = useAgentChat();

  const handleSend = React.useCallback(
    async (message: string) => {
      try {
        await sendMessage(message);
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

  if (!isOpen) return null;

  return (
    <div
      className={cn(
        'fixed bottom-20 right-4 z-50',
        'w-[380px] max-w-[calc(100vw-32px)]',
        'h-[500px] max-h-[calc(100vh-120px)]',
        'flex flex-col',
        'bg-background border border-border rounded-lg shadow-xl',
        'animate-in slide-in-from-bottom-4 fade-in-0 duration-200'
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border/40 bg-muted/30 rounded-t-lg">
        <div className="flex items-center gap-2">
          <div
            className={cn(
              'w-2 h-2 rounded-full',
              isWorking ? 'bg-yellow-500 animate-pulse' : 'bg-green-500'
            )}
          />
          <span className="text-sm font-medium">Dagu Agent</span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClear}
            className="h-7 w-7 p-0"
            title="New conversation"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={closeChat}
            className="h-7 w-7 p-0"
            title="Minimize"
          >
            <Minimize2 className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={closeChat}
            className="h-7 w-7 p-0"
            title="Close"
          >
            <X className="h-3.5 w-3.5" />
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
