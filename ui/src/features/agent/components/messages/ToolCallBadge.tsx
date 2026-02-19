import { useMemo, useState } from 'react';
import { Terminal } from 'lucide-react';
import { cn } from '@/lib/utils';
import { ToolCall } from '../../types';
import { ToolContentViewer } from '../ToolViewers';

function parseToolArguments(jsonString: string): Record<string, unknown> {
  try {
    return JSON.parse(jsonString) as Record<string, unknown>;
  } catch {
    return {};
  }
}

export function ToolCallBadge({ toolCall }: { toolCall: ToolCall }): React.ReactNode {
  const [expanded, setExpanded] = useState(true);
  const args = useMemo(() => parseToolArguments(toolCall.function.arguments), [toolCall.function.arguments]);

  return (
    <div className="rounded border border-border bg-muted dark:bg-surface text-xs overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-secondary transition-colors"
      >
        <Terminal className="h-3 w-3 text-muted-foreground" />
        <span className="font-mono font-medium">{toolCall.function.name}</span>
        <span className="text-muted-foreground ml-auto">{expanded ? '[-]' : '[+]'}</span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border bg-card dark:bg-surface">
          <ToolContentViewer toolName={toolCall.function.name} args={args} />
        </div>
      )}
    </div>
  );
}

export function ToolCallList({ toolCalls, className }: { toolCalls: ToolCall[]; className?: string }): React.ReactNode {
  return (
    <div className={cn('space-y-1', className)}>
      {toolCalls.map((tc) => (
        <ToolCallBadge key={tc.id} toolCall={tc} />
      ))}
    </div>
  );
}
