import ReactDiffViewer, { DiffMethod } from 'react-diff-viewer-continued';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { SyncStatus } from '@/api/v2/schema';
import { useUserPreferences } from '@/contexts/UserPreference';
import { Loader2 } from 'lucide-react';

interface DiffModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dagId: string;
  status?: SyncStatus;
  localContent?: string;
  remoteContent?: string;
  remoteCommit?: string;
  remoteAuthor?: string;
  isLoading?: boolean;
}

export function DiffModal({
  open,
  onOpenChange,
  dagId,
  status,
  localContent,
  remoteContent,
  remoteCommit,
  remoteAuthor,
  isLoading,
}: DiffModalProps) {
  const { preferences } = useUserPreferences();
  const isDarkMode = preferences.theme === 'dark';

  const getTitles = () => {
    if (!status) return { left: 'Remote', right: 'Local' };

    switch (status) {
      case SyncStatus.modified:
        return {
          left: remoteCommit ? `Remote (${remoteCommit.slice(0, 7)})` : 'Remote',
          right: 'Local (modified)',
        };
      case SyncStatus.conflict:
        return {
          left: remoteAuthor ? `Remote (${remoteAuthor})` : 'Remote',
          right: 'Local (conflicting)',
        };
      case SyncStatus.untracked:
        return {
          left: '(new file)',
          right: 'Local',
        };
      case SyncStatus.synced:
        return {
          left: 'Remote',
          right: 'Local (identical)',
        };
      default:
        return { left: 'Remote', right: 'Local' };
    }
  };

  const titles = getTitles();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-6xl max-h-[90vh] overflow-hidden flex flex-col p-0">
        <DialogHeader className="px-4 py-3 border-b border-border/40">
          <DialogTitle className="text-sm font-mono">{dagId}</DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-auto min-h-0">
          {isLoading ? (
            <div className="flex items-center justify-center h-64">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <ReactDiffViewer
              oldValue={remoteContent || ''}
              newValue={localContent || ''}
              splitView={true}
              leftTitle={titles.left}
              rightTitle={titles.right}
              useDarkTheme={isDarkMode}
              compareMethod={DiffMethod.LINES}
              showDiffOnly={false}
              styles={{
                variables: {
                  dark: {
                    diffViewerBackground: '#1e1e1e',
                    gutterBackground: '#252526',
                    addedBackground: '#1e3a29',
                    addedGutterBackground: '#1e3a29',
                    removedBackground: '#3a1e1e',
                    removedGutterBackground: '#3a1e1e',
                    wordAddedBackground: '#2ea043',
                    wordRemovedBackground: '#f85149',
                    emptyLineBackground: '#1e1e1e',
                    gutterColor: '#6e7681',
                  },
                  light: {
                    diffViewerBackground: '#ffffff',
                    gutterBackground: '#f6f8fa',
                    addedBackground: '#e6ffec',
                    addedGutterBackground: '#ccffd8',
                    removedBackground: '#ffebe9',
                    removedGutterBackground: '#ffd7d5',
                    wordAddedBackground: '#abf2bc',
                    wordRemovedBackground: '#ff818266',
                    emptyLineBackground: '#ffffff',
                    gutterColor: '#57606a',
                  },
                },
                contentText: {
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                  fontSize: '12px',
                  lineHeight: '1.5',
                },
                titleBlock: {
                  padding: '8px 12px',
                  fontSize: '12px',
                  fontWeight: 500,
                },
                line: {
                  padding: '0 8px',
                },
              }}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
