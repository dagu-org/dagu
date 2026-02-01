import type { ReactElement } from 'react';

import { Terminal } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

import { useAgentChatContext } from '../context/AgentChatContext';

const BASE_STYLES =
  'fixed bottom-4 right-4 z-50 h-9 px-3 rounded-md shadow-md bg-card border border-border backdrop-blur transition-all duration-200';

export function AgentChatButton(): ReactElement {
  const { isOpen, toggleChat, conversationState } = useAgentChatContext();
  const isWorking = conversationState?.working ?? false;

  return (
    <Button
      variant="outline"
      onClick={toggleChat}
      className={cn(
        BASE_STYLES,
        isOpen && 'opacity-0 pointer-events-none',
        isWorking && 'border-yellow-500/50'
      )}
      title="Agent Console"
    >
      <Terminal className="h-4 w-4 mr-1.5" />
      <span className="text-xs font-medium">Agent</span>
      {isWorking && (
        <span className="ml-1.5 w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" />
      )}
    </Button>
  );
}
