import * as React from 'react';
import { MessageSquare } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useAgentChatContext } from '../context/AgentChatContext';
import { cn } from '@/lib/utils';

export function AgentChatButton() {
  const { isOpen, toggleChat, conversationState } = useAgentChatContext();
  const isWorking = conversationState?.working ?? false;

  return (
    <Button
      onClick={toggleChat}
      className={cn(
        'fixed bottom-4 right-4 z-50',
        'h-12 w-12 rounded-full shadow-lg',
        'transition-all duration-200',
        isOpen && 'opacity-0 pointer-events-none',
        isWorking && 'animate-pulse'
      )}
      title="Open AI Agent"
    >
      <MessageSquare className="h-5 w-5" />
      {isWorking && (
        <span className="absolute -top-1 -right-1 w-3 h-3 bg-yellow-500 rounded-full animate-ping" />
      )}
    </Button>
  );
}
