import { ArrowRight } from 'lucide-react';
import type { NavigateToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function NavigateToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { path } = args as unknown as NavigateToolInput;
  return (
    <div className="flex items-center gap-2 text-xs font-mono">
      <ArrowRight className="h-3 w-3 text-muted-foreground flex-shrink-0" />
      <span className="text-primary hover:underline cursor-pointer truncate" title={path}>
        {path}
      </span>
    </div>
  );
}
