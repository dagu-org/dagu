import * as React from 'react';
import { X, RotateCcw, Terminal } from 'lucide-react';
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
        <div className="flex items-center gap-2">
          <Terminal className="h-4 w-4 text-muted-foreground" />
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Agent Console</span>
          {isWorking && (
            <span className="text-xs text-yellow-500 font-mono">running...</span>
          )}
        </div>
        <div className="flex items-center gap-0.5">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClear}
            className="h-6 w-6 p-0 text-muted-foreground hover:text-foreground"
            title="Clear"
          >
            <RotateCcw className="h-3 w-3" />
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
