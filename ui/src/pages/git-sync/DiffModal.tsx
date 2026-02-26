import ReactDiffViewer, { DiffMethod } from 'react-diff-viewer-continued';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { SyncStatus } from '@/api/v1/schema';
import { useUserPreferences } from '@/contexts/UserPreference';
import { Upload, RotateCcw, Trash2, EyeOff, RefreshCw } from 'lucide-react';

interface DiffModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dagId: string;
  status?: SyncStatus;
  localContent?: string;
  remoteContent?: string;
  remoteCommit?: string;
  remoteAuthor?: string;
  canPublish?: boolean;
  canRevert?: boolean;
  onPublish?: () => void;
  onRevert?: () => void;
  onForget?: () => void;
  onDelete?: () => void;
  isForgetting?: boolean;
  isDeleting?: boolean;
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
  canPublish,
  canRevert,
  onPublish,
  onRevert,
  onForget,
  onDelete,
  isForgetting,
  isDeleting,
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
      case SyncStatus.missing:
        return {
          left: remoteCommit ? `Remote (${remoteCommit.slice(0, 7)})` : 'Remote',
          right: 'Local (missing)',
        };
      default:
        return { left: 'Remote', right: 'Local' };
    }
  };

  const titles = getTitles();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-6xl max-h-[90vh] overflow-hidden flex flex-col p-0 duration-100">
        <DialogHeader className="px-4 py-3 border-b border-border/40">
          <DialogTitle className="text-sm font-mono">{dagId}</DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-auto">
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
        </div>
        {status === SyncStatus.missing && (onForget || onDelete) ? (
          <DialogFooter className="px-4 py-3 border-t border-border/40">
            {onForget && (
              <Button
                variant="outline"
                size="sm"
                onClick={onForget}
                disabled={isForgetting}
              >
                {isForgetting ? (
                  <RefreshCw className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                ) : (
                  <EyeOff className="h-3.5 w-3.5 mr-1.5" />
                )}
                Forget
              </Button>
            )}
            {onDelete && (
              <Button
                variant="destructive"
                size="sm"
                onClick={onDelete}
                disabled={isDeleting}
              >
                {isDeleting ? (
                  <RefreshCw className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                ) : (
                  <Trash2 className="h-3.5 w-3.5 mr-1.5" />
                )}
                Delete from Remote
              </Button>
            )}
          </DialogFooter>
        ) : (canPublish || canRevert) ? (
          <DialogFooter className="px-4 py-3 border-t border-border/40">
            {canRevert && onRevert && (
              <Button
                variant="outline"
                size="sm"
                onClick={onRevert}
                className="text-destructive hover:text-destructive"
              >
                <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
                Revert
              </Button>
            )}
            {canPublish && onPublish && (
              <Button size="sm" onClick={onPublish}>
                <Upload className="h-3.5 w-3.5 mr-1.5" />
                Push
              </Button>
            )}
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
