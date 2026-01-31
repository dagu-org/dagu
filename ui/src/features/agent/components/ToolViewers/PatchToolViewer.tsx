import { FilePlus, FileX } from 'lucide-react';
import { useUserPreferences } from '@/contexts/UserPreference';
import type { PatchToolInput } from '../../types';
import { JsonPatchViewer } from '../JsonPatchViewer';
import { DefaultToolViewer } from './DefaultToolViewer';
import type { ToolViewerProps } from './index';

export function PatchToolViewer({ args, toolName }: ToolViewerProps): React.ReactNode {
  const { preferences } = useUserPreferences();
  const isDark = preferences.theme === 'dark';
  const { path, operation, content, old_string, new_string } = args as unknown as PatchToolInput;

  // Replace operation with old_string and new_string - use existing JsonPatchViewer
  if (old_string !== undefined && new_string !== undefined) {
    return <JsonPatchViewer patch={{ path, old_string, new_string }} />;
  }

  // Create operation - show file creation with content preview (all additions)
  if (operation === 'create' && content !== undefined) {
    const lines = content.split('\n');
    const filename = path?.split('/').pop() || path;

    return (
      <div className="rounded border border-border overflow-hidden text-xs font-mono">
        <div className="flex items-center gap-2 px-2 py-1 bg-muted border-b border-border text-muted-foreground">
          <FilePlus className="h-3 w-3 text-green-600 dark:text-green-400" />
          <span className="truncate" title={path}>{filename}</span>
          <span className="text-green-600 dark:text-green-400">+{lines.length}</span>
        </div>
        <div className="max-h-[300px] overflow-auto">
          {lines.map((line, idx) => (
            <div
              key={idx}
              className="px-2 py-0.5 whitespace-pre"
              style={{
                backgroundColor: isDark ? 'rgba(34,197,94,0.1)' : '#d1fae5',
                color: isDark ? '#4ade80' : '#14532d',
              }}
            >
              <span className="select-none mr-1">+</span>
              {line}
            </div>
          ))}
        </div>
      </div>
    );
  }

  // Delete operation - show file deletion indicator
  if (operation === 'delete') {
    return (
      <div className="flex items-center gap-2 px-2 py-1 text-xs font-mono text-red-600 dark:text-red-400">
        <FileX className="h-3 w-3" />
        <span className="truncate" title={path}>{path}</span>
      </div>
    );
  }

  // Fallback for unknown patch formats
  return <DefaultToolViewer args={args} toolName={toolName} />;
}
