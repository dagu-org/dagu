import { FileText } from 'lucide-react';
import type { ReadToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function ReadToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { path, offset, limit } = args as unknown as ReadToolInput;
  const filename = path?.split('/').pop() || path;
  const hasRange = offset !== undefined || limit !== undefined;

  return (
    <div className="flex items-center gap-2 text-xs font-mono">
      <FileText className="h-3 w-3 text-muted-foreground flex-shrink-0" />
      <span className="truncate" title={path}>{filename}</span>
      {hasRange && (
        <span className="text-muted-foreground text-[10px] flex-shrink-0">
          {offset !== undefined && `from ${offset}`}
          {offset !== undefined && limit !== undefined && ', '}
          {limit !== undefined && `limit ${limit}`}
        </span>
      )}
    </div>
  );
}
