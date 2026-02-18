import { useState } from 'react';
import { CheckCircle, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { ToolResult } from '../../types';
import { TOOL_RESULT_PREVIEW_LENGTH } from '../../constants';

function truncateContent(content: string, maxLength: number): string {
  if (content.length <= maxLength) return content;
  return content.substring(0, maxLength) + '...';
}

function ToolResultItem({ result }: { result: ToolResult }): React.ReactNode {
  const [expanded, setExpanded] = useState(false);
  const content = result.content ?? '';
  const preview = truncateContent(content, TOOL_RESULT_PREVIEW_LENGTH);

  const StatusIcon = result.is_error ? XCircle : CheckCircle;
  const statusColor = result.is_error ? 'text-red-500' : 'text-green-500';
  const borderStyle = result.is_error
    ? 'border-red-500/40 bg-red-500/10'
    : 'border-green-500/40 bg-green-500/10';

  return (
    <div className={cn('rounded border text-xs overflow-hidden', borderStyle)}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-accent transition-colors text-left"
      >
        <StatusIcon className={cn('h-3 w-3 flex-shrink-0', statusColor)} />
        <span className="font-mono truncate flex-1">
          {expanded ? 'Result' : preview}
        </span>
        <span className="text-muted-foreground ml-1 flex-shrink-0">
          {expanded ? '[-]' : '[+]'}
        </span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border bg-card dark:bg-surface">
          <pre className="text-xs overflow-x-auto whitespace-pre-wrap break-words max-h-[200px] overflow-y-auto">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}

export function ToolResultMessage({ toolResults }: { toolResults: ToolResult[] }): React.ReactNode {
  return (
    <div className="pl-5 space-y-1">
      {toolResults.map((tr) => (
        <ToolResultItem key={tr.tool_use_id} result={tr} />
      ))}
    </div>
  );
}
