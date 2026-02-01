import type { BashToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function BashToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { command, timeout } = args as unknown as BashToolInput;
  return (
    <div className="flex items-start gap-2 text-xs font-mono">
      <span className="text-green-600 dark:text-green-400 flex-shrink-0">$</span>
      <span className="flex-1 break-all whitespace-pre-wrap">{command}</span>
      {timeout && (
        <span className="text-muted-foreground text-[10px] flex-shrink-0">{timeout}ms</span>
      )}
    </div>
  );
}
