import { ReactElement, useCallback, useEffect, useRef, useState } from 'react';
import { CheckCircle2, Loader2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
import {
  ANIMATION_CLOSE_DURATION_MS,
  ANIMATION_OPEN_DURATION_MS,
  DELEGATE_PANEL_GAP,
  DELEGATE_PANEL_HEIGHT,
  DELEGATE_PANEL_MARGIN,
  DELEGATE_PANEL_MIN_HEIGHT,
  DELEGATE_PANEL_MIN_WIDTH,
  DELEGATE_PANEL_WIDTH,
  TASK_TRUNCATE_LENGTH,
} from '../constants';
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
  const cols = Math.max(1, Math.floor((window.innerWidth - DELEGATE_PANEL_MARGIN) / (DELEGATE_PANEL_WIDTH + DELEGATE_PANEL_GAP)));
  const row = Math.floor(index / cols);
  const col = index % cols;
  const left = DELEGATE_PANEL_MARGIN + col * (DELEGATE_PANEL_WIDTH + DELEGATE_PANEL_GAP);
  const top = DELEGATE_PANEL_MARGIN + row * (DELEGATE_PANEL_HEIGHT + DELEGATE_PANEL_GAP);

  const { bounds, dragHandlers, resizeHandlers } = useResizableDraggable({
    defaultWidth: DELEGATE_PANEL_WIDTH,
    defaultHeight: DELEGATE_PANEL_HEIGHT,
    defaultRight: Math.max(0, window.innerWidth - left - DELEGATE_PANEL_WIDTH),
    defaultBottom: Math.max(0, window.innerHeight - top - DELEGATE_PANEL_HEIGHT),
    minWidth: DELEGATE_PANEL_MIN_WIDTH,
    minHeight: DELEGATE_PANEL_MIN_HEIGHT,
    storageKey: 'delegate-panel-bounds',
  });

  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const handleClose = useCallback(() => {
    setIsClosing(true);
    setTimeout(() => onCloseRef.current(), ANIMATION_CLOSE_DURATION_MS);
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

  const truncatedTask = task.length > TASK_TRUNCATE_LENGTH ? task.slice(0, TASK_TRUNCATE_LENGTH) + '...' : task;
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
          ? `agent-modal-out ${ANIMATION_CLOSE_DURATION_MS}ms ease-in forwards`
          : `delegate-panel-in ${ANIMATION_OPEN_DURATION_MS}ms ease-out`,
      }}
      onMouseDown={onBringToFront}
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
