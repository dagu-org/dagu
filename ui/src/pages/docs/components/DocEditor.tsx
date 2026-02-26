import { useCanWrite } from '@/contexts/AuthContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { useDocContentWithConflictDetection } from '@/hooks/useDocContentWithConflictDetection';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { useErrorModal } from '@/components/ui/error-modal';
import MarkdownEditor from '@/components/editors/MarkdownEditor';
import { DocWysiwygEditor } from './DocWysiwygEditor';
import { Button } from '@/components/ui/button';
import { FileText, Loader2, Save } from 'lucide-react';
import { cn } from '@/lib/utils';
import React, { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { DocExternalChangeDialog } from './DocExternalChangeDialog';

interface DocEditorProps {
  docPath: string;
}

export function DocEditor({ docPath }: DocEditorProps) {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const canWrite = useCanWrite();
  const { showToast } = useSimpleToast();
  const { showError } = useErrorModal();

  const [editorMode, setEditorMode] = useState<'raw' | 'rich'>(() => {
    return (localStorage.getItem('doc-editor-mode') as 'raw' | 'rich') || 'raw';
  });

  const {
    shouldUseFallback,
    doc,
    currentValue,
    setCurrentValue,
    hasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
  } = useDocContentWithConflictDetection({ docPath });

  // Fallback: fetch via REST if SSE fails
  const [fallbackLoaded, setFallbackLoaded] = useState(false);
  useEffect(() => {
    if (shouldUseFallback && !fallbackLoaded) {
      (async () => {
        try {
          const { data } = await client.GET('/docs/{docPath}', {
            params: { path: { docPath }, query: { remoteNode } },
          });
          if (data) {
            setCurrentValue(data.content);
            setFallbackLoaded(true);
          }
        } catch {
          // Best-effort
        }
      })();
    }
  }, [shouldUseFallback, fallbackLoaded, docPath, client, remoteNode, setCurrentValue]);

  // Reset fallback state on docPath change
  useEffect(() => {
    setFallbackLoaded(false);
  }, [docPath]);

  const saveHandlerRef = useRef<(() => Promise<void>) | null>(null);

  const handleSave = useCallback(async () => {
    const { error } = await client.PATCH('/docs/{docPath}', {
      params: { path: { docPath }, query: { remoteNode } },
      body: { content: currentValue },
    });

    if (error) {
      showError('Failed to save document', 'Please try again.');
      return;
    }

    markAsSaved(currentValue);
    showToast('Document saved');
  }, [currentValue, docPath, remoteNode, client, markAsSaved, showToast, showError]);

  useEffect(() => {
    saveHandlerRef.current = handleSave;
  }, [handleSave]);

  // Ctrl+S / Cmd+S
  useEffect(() => {
    if (!canWrite) return;

    const handleKeyDown = async (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 's') {
        event.preventDefault();
        if (saveHandlerRef.current) {
          await saveHandlerRef.current();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [canWrite]);

  // Persist editor mode
  useEffect(() => {
    localStorage.setItem('doc-editor-mode', editorMode);
  }, [editorMode]);

  // Track whether initial content has been loaded (via SSE or REST fallback)
  // so the WYSIWYG editor only mounts once content is available.
  const contentLoaded = doc != null || fallbackLoaded;

  const title = doc?.title || docPath.split('/').pop() || docPath;

  return (
    <div className="flex flex-col h-full">
      {/* Conflict dialog */}
      <DocExternalChangeDialog
        visible={conflict.hasConflict}
        onDiscard={() => resolveConflict('discard')}
        onIgnore={() => resolveConflict('ignore')}
      />

      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <FileText className="w-4 h-4 text-muted-foreground shrink-0" />
          <span className="text-sm font-medium truncate">{title}</span>
          {hasUnsavedChanges && (
            <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" title="Unsaved changes" />
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {/* Mode toggle */}
          <div className="flex items-center rounded-md border border-border text-xs">
            <button
              onClick={() => setEditorMode('raw')}
              className={cn(
                'px-2 py-1 rounded-l-md transition-colors',
                editorMode === 'raw'
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              Raw
            </button>
            <button
              onClick={() => setEditorMode('rich')}
              className={cn(
                'px-2 py-1 rounded-r-md transition-colors',
                editorMode === 'rich'
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              Rich
            </button>
          </div>
          {canWrite && (
            <Button
              size="sm"
              disabled={!hasUnsavedChanges}
              onClick={handleSave}
              title="Save (Ctrl+S / Cmd+S)"
            >
              <Save className="h-4 w-4" />
              Save
            </Button>
          )}
        </div>
      </div>

      {/* Editor */}
      <div className="flex-1 min-h-0">
        {editorMode === 'raw' ? (
          <MarkdownEditor
            value={currentValue}
            onChange={canWrite ? (v) => setCurrentValue(v || '') : undefined}
            readOnly={!canWrite}
          />
        ) : !contentLoaded ? (
          <div className="flex items-center justify-center h-full">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <DocWysiwygEditor
            value={currentValue}
            onChange={canWrite ? (v) => setCurrentValue(v) : undefined}
            readOnly={!canWrite}
          />
        )}
      </div>
    </div>
  );
}
