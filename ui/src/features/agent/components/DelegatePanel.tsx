import { ReactElement, useCallback, useEffect, useRef, useState } from 'react';
import { CheckCircle2, Loader2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { Message } from '../types';
import { ChatMessages } from './ChatMessages';
import { ResizeHandles } from './ResizeHandles';

interface DelegatePanelProps {
  delegateId: string;
  task: string;
  status: 'running' | 'completed';
  zIndex: number;
  index: number;
  messages: Message[];
  onClose: () => void;
  onBringToFront: () => void;
}

export function DelegatePanel({
  delegateId,
  task,
  status,
  zIndex,
  index,
  messages,
  onClose,
  onBringToFront,
}: DelegatePanelProps): ReactElement {
  const [isClosing, setIsClosing] = useState(false);

  // Grid layout: flow from top-left, wrap at window edge
  const PANEL_W = 320;
  const PANEL_H = 360;
  const GAP = 12;
  const MARGIN = 16;
  const cols = Math.max(1, Math.floor((window.innerWidth - MARGIN) / (PANEL_W + GAP)));
  const row = Math.floor(index / cols);
  const col = index % cols;
  const left = MARGIN + col * (PANEL_W + GAP);
  const top = MARGIN + row * (PANEL_H + GAP);

  const { bounds, dragHandlers, resizeHandlers } = useResizableDraggable({
    defaultWidth: PANEL_W,
    defaultHeight: PANEL_H,
    defaultRight: Math.max(0, window.innerWidth - left - PANEL_W),
    defaultBottom: Math.max(0, window.innerHeight - top - PANEL_H),
    minWidth: 280,
    minHeight: 200,
    storageKey: `delegate-panel-${delegateId}`,
  });

  const handleMouseDown = useCallback(() => {
    onBringToFront();
  }, [onBringToFront]);

  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const handleClose = useCallback(() => {
    setIsClosing(true);
    setTimeout(() => onCloseRef.current(), 250);
  }, []);

  // Auto-close with CRT animation when delegate finishes (running → completed)
  const wasRunningRef = useRef(status === 'running');
  useEffect(() => {
    if (status === 'running') {
      wasRunningRef.current = true;
    } else if (status === 'completed' && wasRunningRef.current) {
      wasRunningRef.current = false;
      handleClose();
    }
  }, [status, handleClose]);

  const truncatedTask = task.length > 40 ? task.slice(0, 40) + '...' : task;
  const isRunning = status === 'running';

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
          ? 'agent-modal-out 250ms ease-in forwards'
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
          className="h-8 w-8 p-0 flex items-center justify-center text-muted-foreground hover:text-foreground flex-shrink-0"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Body - messages */}
      <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
        <ChatMessages
          messages={messages}
          pendingUserMessage={null}
          isWorking={isRunning}
        />
      </div>
    </div>
  );
}
