import { HelpCircle } from 'lucide-react';
import type { ToolViewerProps } from './index';

export function AskUserToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { question, options, multi_select } = args as Record<string, unknown>;
  const optionList = Array.isArray(options) ? options : [];
  return (
    <div className="space-y-1.5 text-xs">
      <div className="flex items-start gap-2">
        <HelpCircle className="h-3 w-3 text-muted-foreground flex-shrink-0 mt-0.5" />
        <span className="whitespace-pre-wrap break-words">{String(question ?? '')}</span>
      </div>
      {optionList.length > 0 && (
        <div className="flex flex-wrap gap-1 pl-5">
          {optionList.map((opt, idx) => {
            const label = typeof opt === 'string' ? opt : opt?.label ?? '';
            const description = typeof opt === 'string' ? undefined : opt?.description;
            return (
              <span
                key={idx}
                className="inline-flex items-center px-1.5 py-0.5 rounded bg-muted text-muted-foreground text-[10px]"
                title={description}
              >
                {multi_select && <span className="mr-0.5">☐</span>}
                {label}
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
