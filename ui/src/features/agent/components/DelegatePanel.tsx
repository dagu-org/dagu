import { ReactElement, useCallback, useState } from 'react';
import { CheckCircle2, Loader2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useDelegateStream } from '../hooks/useDelegateStream';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { ChatMessages } from './ChatMessages';
import { ResizeHandles } from './ResizeHandles';

interface DelegatePanelProps {
  delegateId: string;
  task: string;
  status: 'running' | 'completed';
  zIndex: number;
  index: number;
  onClose: () => void;
  onBringToFront: () => void;
}

export function DelegatePanel({
  delegateId,
  task,
  status,
  zIndex,
  index,
  onClose,
  onBringToFront,
}: DelegatePanelProps): ReactElement {
  const { messages, isWorking } = useDelegateStream(delegateId);
  const [isClosing, setIsClosing] = useState(false);
  const { bounds, dragHandlers, resizeHandlers } = useResizableDraggable({
    defaultWidth: 320,
    defaultHeight: 360,
    defaultRight: 468 + index * 332,
    defaultBottom: 64,
    minWidth: 280,
    minHeight: 200,
    storageKey: `delegate-panel-${delegateId}`,
  });

  const handleMouseDown = useCallback(() => {
    onBringToFront();
  }, [onBringToFront]);

  const handleClose = useCallback(() => {
    setIsClosing(true);
    setTimeout(() => onClose(), 150);
  }, [onClose]);

  const truncatedTask = task.length > 40 ? task.slice(0, 40) + '...' : task;
  const isRunning = status === 'running' || isWorking;

  return (
    <div
      className={cn(
        'fixed',
        'flex flex-col',
        'bg-card dark:bg-zinc-950 border border-border-strong dark:border-border rounded-lg overflow-hidden',
        'shadow-lg'
      )}
      style={{
        right: bounds.right,
        bottom: bounds.bottom,
        width: bounds.width,
        height: bounds.height,
        zIndex,
        animation: isClosing
          ? 'delegate-panel-out 150ms ease-in forwards'
          : 'delegate-panel-in 250ms ease-out',
      }}
      onMouseDown={handleMouseDown}
    >
      <ResizeHandles resizeHandlers={resizeHandlers} />
      {/* Title bar */}
      <div
        className={cn(
          'flex items-center gap-1.5 px-2 h-8 min-h-[32px]',
          'bg-secondary dark:bg-zinc-900 border-b border-border-strong dark:border-border',
          'cursor-move select-none'
        )}
        {...dragHandlers}
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
          onClick={(e) => { e.stopPropagation(); handleClose(); }}
          onMouseDown={(e) => e.stopPropagation()}
          className="text-muted-foreground hover:text-foreground flex-shrink-0 p-0.5"
        >
          <X className="h-3 w-3" />
        </button>
      </div>

      {/* Body - messages */}
      <div className="flex-1 min-h-0 overflow-hidden">
        <ChatMessages
          messages={messages}
          pendingUserMessage={null}
          isWorking={isRunning}
        />
      </div>
    </div>
  );
}
