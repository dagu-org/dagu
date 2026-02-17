import type { ReactElement } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { CompletedDelegateInfo } from '../types';
import { formatCost } from '../utils/formatCost';

interface Props {
  delegates: CompletedDelegateInfo[];
  onReopen: (id: string, task: string) => void;
}

export function CompletedDelegatesList({ delegates, onReopen }: Props): ReactElement | null {
  if (delegates.length === 0) return null;

  return (
    <div className="flex items-center gap-1 px-2 py-1 border-b border-border bg-muted/30 overflow-x-auto">
      <span className="text-[10px] text-muted-foreground flex-shrink-0">Sub-agents:</span>
      {delegates.map((d) => (
        <button
          key={d.id}
          onClick={() => onReopen(d.id, d.task)}
          className={cn(
            'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px]',
            'bg-green-500/10 border border-green-500/20 text-foreground',
            'hover:bg-green-500/20 transition-colors cursor-pointer',
            'max-w-[150px]'
          )}
          title={d.task}
        >
          <CheckCircle2 className="h-2.5 w-2.5 text-green-500 flex-shrink-0" />
          <span className="truncate">
            {d.task.length > 25 ? d.task.slice(0, 25) + '...' : d.task}
          </span>
          {d.cost != null && d.cost > 0 && (
            <span className="text-muted-foreground/60 flex-shrink-0">{formatCost(d.cost)}</span>
          )}
        </button>
      ))}
    </div>
  );
}
