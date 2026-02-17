import { ReactElement, useState } from 'react';
import { CheckCircle2, Loader2, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useDelegateStream } from '../hooks/useDelegateStream';
import { DelegateInfo } from '../types';
import { formatCost } from '../utils/formatCost';
import { ChatMessages } from './ChatMessages';

interface InlineDelegateResultProps {
  delegateId: string;
  delegateInfo?: DelegateInfo;
}

export function InlineDelegateResult({
  delegateId,
  delegateInfo,
}: InlineDelegateResultProps): ReactElement {
  const { messages, isWorking } = useDelegateStream(delegateId);
  const [expanded, setExpanded] = useState(false);

  const task = delegateInfo?.task ?? 'sub-agent';
  const isRunning = delegateInfo?.status === 'running' || isWorking;
  const isError = !isRunning && messages.some((m) => m.type === 'error');
  const cost = delegateInfo?.cost;

  const truncatedTask = task.length > 60 ? task.slice(0, 60) + '...' : task;

  // Determine status styling
  let StatusIcon = Loader2;
  let borderColor = 'border-blue-500/40';
  let bgColor = 'bg-blue-500/5';
  let iconColor = 'text-blue-500';
  let iconClass = 'animate-spin';

  if (!isRunning && !isError) {
    StatusIcon = CheckCircle2;
    borderColor = 'border-green-500/40';
    bgColor = 'bg-green-500/5';
    iconColor = 'text-green-500';
    iconClass = '';
  } else if (isError) {
    StatusIcon = XCircle;
    borderColor = 'border-red-500/40';
    bgColor = 'bg-red-500/5';
    iconColor = 'text-red-500';
    iconClass = '';
  }

  // Get last assistant message content for collapsed preview
  const lastAssistantMsg = [...messages].reverse().find((m) => m.type === 'assistant' && m.content);
  const preview = lastAssistantMsg?.content
    ? lastAssistantMsg.content.length > 120
      ? lastAssistantMsg.content.slice(0, 120) + '...'
      : lastAssistantMsg.content
    : isRunning
      ? 'processing...'
      : 'No output';

  return (
    <div className={cn('rounded border text-xs overflow-hidden', borderColor, bgColor)}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-accent/50 transition-colors text-left"
      >
        <StatusIcon className={cn('h-3 w-3 flex-shrink-0', iconColor, iconClass)} />
        <span className="text-[10px] text-muted-foreground flex-shrink-0">sub-agent</span>
        <span className="font-mono truncate flex-1">{truncatedTask}</span>
        {cost != null && cost > 0 && (
          <span className="text-[10px] text-muted-foreground flex-shrink-0">{formatCost(cost)}</span>
        )}
        <span className="text-muted-foreground ml-1 flex-shrink-0">
          {expanded ? '[-]' : '[+]'}
        </span>
      </button>
      {!expanded && (
        <div className="px-2 py-1 border-t border-border/50 text-muted-foreground">
          <p className="truncate">{preview}</p>
        </div>
      )}
      {expanded && (
        <div className="border-t border-border/50 max-h-[300px] overflow-hidden flex flex-col">
          <ChatMessages
            messages={messages}
            pendingUserMessage={null}
            isWorking={isRunning}
          />
        </div>
      )}
    </div>
  );
}
