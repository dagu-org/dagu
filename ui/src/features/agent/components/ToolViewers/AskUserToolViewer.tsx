import { HelpCircle } from 'lucide-react';
import type { ToolViewerProps } from './index';

export function AskUserToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { question, options, multi_select } = args as Record<string, unknown>;
  const isMultiSelect = multi_select === true;
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
            const rawLabel =
              typeof opt === 'string'
                ? opt
                : opt && typeof opt === 'object' && 'label' in opt
                  ? (opt as { label?: unknown }).label
                  : '';
            const rawDescription =
              typeof opt === 'string'
                ? undefined
                : opt && typeof opt === 'object' && 'description' in opt
                  ? (opt as { description?: unknown }).description
                  : undefined;
            const label = String(rawLabel ?? '');
            const description = rawDescription == null ? undefined : String(rawDescription);
            return (
              <span
                key={idx}
                className="inline-flex items-center px-1.5 py-0.5 rounded bg-muted text-muted-foreground text-[10px]"
                title={description}
              >
                {isMultiSelect && <span className="mr-0.5">☐</span>}
                {label}
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
