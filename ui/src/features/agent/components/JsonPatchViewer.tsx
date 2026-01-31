import { useMemo } from 'react';
import { useUserPreferences } from '@/contexts/UserPreference';
import { type JsonPatch, type DiffLine, computeDiffLines } from '../utils/diffUtils';

interface JsonPatchViewerProps {
  patch: JsonPatch;
}

export function JsonPatchViewer({ patch }: JsonPatchViewerProps): React.ReactNode {
  const { preferences } = useUserPreferences();
  const isDark = preferences.theme === 'dark';

  const diffLines = useMemo(
    () => computeDiffLines(patch.old_string, patch.new_string),
    [patch.old_string, patch.new_string]
  );

  const stats = useMemo(() => {
    let additions = 0;
    let deletions = 0;
    for (const line of diffLines) {
      if (line.type === 'addition') additions++;
      if (line.type === 'deletion') deletions++;
    }
    return { additions, deletions };
  }, [diffLines]);

  return (
    <div className="rounded border border-border overflow-hidden text-xs font-mono">
      {/* Header */}
      <div className="flex items-center gap-2 px-2 py-1 bg-muted border-b border-border text-muted-foreground">
        {patch.path && (
          <span className="truncate">{patch.path.split('/').pop()}</span>
        )}
        <span className="text-green-600 dark:text-green-400">+{stats.additions}</span>
        <span className="text-red-600 dark:text-red-400">-{stats.deletions}</span>
      </div>

      {/* Diff lines */}
      <div className="max-h-[300px] overflow-auto">
        <div className="min-w-fit">
          {diffLines.map((line, idx) => (
            <DiffLineRow key={idx} line={line} isDark={isDark} />
          ))}
        </div>
      </div>
    </div>
  );
}

function DiffLineRow({ line, isDark }: { line: DiffLine; isDark: boolean }): React.ReactNode {
  const prefix = {
    addition: '+',
    deletion: '-',
    context: ' ',
  };

  const bgColor = {
    addition: '#d1fae5', // green-100
    deletion: '#fee2e2', // red-100
    context: 'transparent',
  };

  const darkBgColor = {
    addition: 'rgba(34, 197, 94, 0.1)',
    deletion: 'rgba(239, 68, 68, 0.1)',
    context: 'transparent',
  };

  const textColor = {
    addition: '#14532d', // green-900
    deletion: '#7f1d1d', // red-900
    context: 'inherit',
  };

  const darkTextColor = {
    addition: '#4ade80', // green-400
    deletion: '#f87171', // red-400
    context: 'inherit',
  };

  return (
    <div
      className="px-2 py-0.5 whitespace-pre"
      style={{
        backgroundColor: isDark ? darkBgColor[line.type] : bgColor[line.type],
        color: isDark ? darkTextColor[line.type] : textColor[line.type],
      }}
    >
      <span className="select-none mr-1">{prefix[line.type]}</span>
      {line.content}
    </div>
  );
}
