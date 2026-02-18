import { useMemo } from 'react';
import { CheckCircle, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { DelegateInfo, ToolCall } from '../../types';
import { TASK_TRUNCATE_LENGTH } from '../../constants';

function parseDelegateTasks(toolCall: ToolCall): string[] {
  try {
    const args = JSON.parse(toolCall.function.arguments);
    if (Array.isArray(args.tasks)) {
      return args.tasks.map((t: { task?: string }) => t.task || '').filter(Boolean);
    }
  } catch { /* ignore */ }
  return [];
}

export function SubAgentChips({
  toolCall,
  delegateStatuses,
  onOpenDelegate,
  isCompleted,
  delegateIds,
}: {
  toolCall: ToolCall;
  delegateStatuses?: Record<string, DelegateInfo>;
  onOpenDelegate?: (id: string) => void;
  isCompleted: boolean;
  delegateIds?: string[];
}): React.ReactNode {
  const tasks = useMemo(() => parseDelegateTasks(toolCall), [toolCall]);

  if (tasks.length === 0) return null;

  return (
    <div className="pl-4 space-y-0.5">
      {tasks.map((task, i) => {
        const delegateId = delegateIds?.[i];
        const delegate = delegateId
          ? delegateStatuses?.[delegateId]
          : undefined;
        const isRunning = !isCompleted && (!delegate || delegate.status === 'running');
        const canClick = !!delegate;

        return (
          <button
            key={i}
            onClick={() => canClick && onOpenDelegate?.(delegate!.id)}
            disabled={!canClick}
            className={cn(
              'flex items-center gap-1.5 px-2 py-1 rounded text-xs max-w-full',
              'border transition-colors',
              isRunning
                ? 'border-orange-500/30 bg-orange-500/5 text-foreground'
                : 'border-green-500/30 bg-green-500/5 text-foreground hover:bg-green-500/10 cursor-pointer',
              !canClick && 'cursor-default'
            )}
          >
            {isRunning
              ? <Loader2 className="h-3 w-3 text-orange-600 dark:text-orange-400 animate-spin flex-shrink-0" />
              : <CheckCircle className="h-3 w-3 text-green-500 flex-shrink-0" />}
            <span className="truncate">{task.length > TASK_TRUNCATE_LENGTH ? task.slice(0, TASK_TRUNCATE_LENGTH) + '...' : task}</span>
          </button>
        );
      })}
    </div>
  );
}
