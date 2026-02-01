import { Search } from 'lucide-react';
import type { ToolViewerProps } from './index';

interface WebSearchInput {
  query?: string;
  max_results?: number;
}

export function WebSearchToolViewer({ args }: ToolViewerProps): React.ReactNode {
  const { query, max_results } = args as unknown as WebSearchInput;

  return (
    <div className="flex items-center gap-2 px-2 py-1 text-xs font-mono">
      <Search className="h-3 w-3 text-blue-600 dark:text-blue-400 shrink-0" />
      <span className="truncate text-muted-foreground" title={query}>
        {query || 'searching...'}
      </span>
      {max_results && (
        <span className="text-muted-foreground shrink-0">({max_results})</span>
      )}
    </div>
  );
}
