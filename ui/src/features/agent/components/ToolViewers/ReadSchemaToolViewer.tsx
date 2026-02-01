import { Database } from 'lucide-react';
import type { ReadSchemaToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function ReadSchemaToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { schema, path } = args as unknown as ReadSchemaToolInput;
  return (
    <div className="flex items-center gap-2 text-xs font-mono">
      <Database className="h-3 w-3 text-muted-foreground flex-shrink-0" />
      <span className="font-medium">{schema}</span>
      {path && (
        <span className="text-muted-foreground truncate" title={path}>
          → {path}
        </span>
      )}
    </div>
  );
}
