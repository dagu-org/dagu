import { Brain } from 'lucide-react';
import type { ThinkToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function ThinkToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { thought } = args as unknown as ThinkToolInput;
  return (
    <div className="flex items-start gap-2 text-xs">
      <Brain className="h-3 w-3 text-muted-foreground flex-shrink-0 mt-0.5" />
      <span className="italic text-muted-foreground whitespace-pre-wrap break-words">{thought}</span>
    </div>
  );
}
