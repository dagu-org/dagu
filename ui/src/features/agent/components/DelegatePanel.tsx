import { ReactElement, useCallback } from 'react';
import { CheckCircle2, ChevronDown, ChevronUp, Loader2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useDelegateStream } from '../hooks/useDelegateStream';
import { ChatMessages } from './ChatMessages';

interface DelegatePanelProps {
  delegateId: string;
  task: string;
  status: 'running' | 'completed';
  minimized: boolean;
  zIndex: number;
  index: number;
  onClose: () => void;
  onBringToFront: () => void;
  onToggleMinimize: () => void;
}

export function DelegatePanel({
  delegateId,
  task,
  status,
  minimized,
  zIndex,
  index,
  onClose,
  onBringToFront,
  onToggleMinimize,
}: DelegatePanelProps): ReactElement {
  const { messages, isWorking } = useDelegateStream(delegateId);

  const handleMouseDown = useCallback(() => {
    onBringToFront();
  }, [onBringToFront]);

  const truncatedTask = task.length > 40 ? task.slice(0, 40) + '...' : task;
  const isRunning = status === 'running' || isWorking;

  return (
    <div
      className={cn(
        'fixed',
        'flex flex-col',
        'bg-popover dark:bg-zinc-950 border border-border rounded-lg overflow-hidden',
        'shadow-lg dark:shadow-[0_0_20px_rgba(0,0,0,0.5)]',
        'transition-[height] duration-150 ease-in-out'
      )}
      style={{
        right: 468,
        bottom: 64 + index * (minimized ? 40 : 200),
        width: 320,
        height: minimized ? 32 : 360,
        zIndex,
      }}
      onMouseDown={handleMouseDown}
    >
      {/* Title bar */}
      <div
        className={cn(
          'flex items-center gap-1.5 px-2 h-8 min-h-[32px]',
          'bg-muted/50 dark:bg-zinc-900 border-b border-border',
          'cursor-pointer select-none'
        )}
        onClick={onToggleMinimize}
      >
        {isRunning ? (
          <Loader2 className="h-3 w-3 text-blue-500 animate-spin flex-shrink-0" />
        ) : (
          <CheckCircle2 className="h-3 w-3 text-green-500 flex-shrink-0" />
        )}
        <span className="text-xs font-medium truncate flex-1 text-foreground">
          {truncatedTask}
        </span>
        <button
          onClick={(e) => { e.stopPropagation(); onToggleMinimize(); }}
          className="text-muted-foreground hover:text-foreground flex-shrink-0 p-0.5"
        >
          {minimized ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onClose(); }}
          className="text-muted-foreground hover:text-foreground flex-shrink-0 p-0.5"
        >
          <X className="h-3 w-3" />
        </button>
      </div>

      {/* Body - messages */}
      {!minimized && (
        <div className="flex-1 min-h-0 overflow-hidden">
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
