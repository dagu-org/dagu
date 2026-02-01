import { HelpCircle } from 'lucide-react';
import type { AskUserToolInput } from '../../types';
import type { ToolViewerProps } from './index';

export function AskUserToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { question, options, multi_select } = args as unknown as AskUserToolInput;
  return (
    <div className="space-y-1.5 text-xs">
      <div className="flex items-start gap-2">
        <HelpCircle className="h-3 w-3 text-muted-foreground flex-shrink-0 mt-0.5" />
        <span className="whitespace-pre-wrap break-words">{question}</span>
      </div>
      {options && options.length > 0 && (
        <div className="flex flex-wrap gap-1 pl-5">
          {options.map((opt, idx) => (
            <span
              key={idx}
              className="inline-flex items-center px-1.5 py-0.5 rounded bg-muted text-muted-foreground text-[10px]"
              title={opt.description}
            >
              {multi_select && <span className="mr-0.5">☐</span>}
              {opt.label}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
