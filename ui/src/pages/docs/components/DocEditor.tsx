import MarkdownEditor from '@/components/editors/MarkdownEditor';
import { MermaidBlock } from '@/components/ui/mermaid-block';
import { useSimpleToast } from '@/components/ui/simple-toast';
import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import './DocPreview.css';
import { useCanWrite } from '@/contexts/AuthContext';
import { useDocTabContext } from '@/contexts/DocTabContext';
import { useClient, useQuery } from '@/hooks/api';
import { useContentEditor } from '@/hooks/useContentEditor';
import { useDocSSE } from '@/hooks/useDocSSE';
import { cn } from '@/lib/utils';
import { AppBarContext } from '@/contexts/AppBarContext';
import { FileText, Save } from 'lucide-react';
import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import DocExternalChangeDialog from './DocExternalChangeDialog';

type Props = {
  tabId: string;
  docPath: string;
};

function DocEditor({ tabId, docPath }: Props) {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const canWrite = useCanWrite();
  const { showToast } = useSimpleToast();
  const {
    getDraft,
    setDraft,
    clearDraft,
    markTabUnsaved,
    markTabSaved,
  } = useDocTabContext();

  // SSE connection for real-time doc data
  const sseResult = useDocSSE(docPath);

  // Polling fallback when SSE fails
  const { data: pollingData } = useQuery(
    '/docs/doc',
    {
      params: {
        query: {
          remoteNode,
          path: docPath,
        },
      },
    },
    {
      revalidateIfStale: sseResult.shouldUseFallback,
      revalidateOnFocus: sseResult.shouldUseFallback,
      revalidateOnMount: true,
      refreshInterval: sseResult.shouldUseFallback ? 5000 : 0,
      isPaused: () => !sseResult.shouldUseFallback && sseResult.isConnected,
    }
  );

  // Best available doc data — prefer SSE when connected, polling when not
  const doc = sseResult.isConnected && sseResult.data ? sseResult.data : pollingData;
  const serverContent = doc?.content ?? null;

  // Change tracking (source-agnostic)
  const {
    currentValue,
    setCurrentValue,
    hasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
  } = useContentEditor({
    key: docPath,
    serverContent,
  });

  const [mode, setMode] = useState<'edit' | 'preview'>(() => {
    const stored = localStorage.getItem('doc-editor-mode');
    return stored === 'preview' ? 'preview' : 'edit';
  });
  const [isSaving, setIsSaving] = useState(false);

  // Use refs for cleanup to avoid stale closures
  const currentValueRef = useRef(currentValue);
  currentValueRef.current = currentValue;
  const hasUnsavedChangesRef = useRef(hasUnsavedChanges);
  hasUnsavedChangesRef.current = hasUnsavedChanges;

  // Restore draft on mount
  useEffect(() => {
    const draft = getDraft(tabId);
    if (draft !== undefined) {
      setCurrentValue(draft);
      clearDraft(tabId);
    }
  }, []); // Only on mount

  // Save draft on unmount (tab switch)
  useEffect(() => {
    return () => {
      if (hasUnsavedChangesRef.current) {
        setDraft(tabId, currentValueRef.current);
      }
    };
  }, [tabId, setDraft]); // Only runs cleanup on tab change or unmount

  // Sync unsaved state to tab context
  useEffect(() => {
    if (hasUnsavedChanges) {
      markTabUnsaved(tabId);
    } else {
      markTabSaved(tabId);
    }
  }, [hasUnsavedChanges, tabId, markTabUnsaved, markTabSaved]);

  // Persist mode preference
  useEffect(() => {
    localStorage.setItem('doc-editor-mode', mode);
  }, [mode]);

  const handleSave = useCallback(async () => {
    if (isSaving || !hasUnsavedChangesRef.current) return;
    setIsSaving(true);
    try {
      const { error } = await client.PATCH('/docs/doc', {
        params: { query: { remoteNode, path: docPath } },
        body: { content: currentValueRef.current },
      });
      if (error) {
        showToast('Failed to save document');
      } else {
        markAsSaved(currentValueRef.current);
        markTabSaved(tabId);
        clearDraft(tabId);
        showToast('Document saved');
      }
    } catch {
      showToast('Failed to save document');
    } finally {
      setIsSaving(false);
    }
  }, [isSaving, client, remoteNode, docPath, markAsSaved, markTabSaved, clearDraft, tabId, showToast]);

  // Keep save handler in ref for keyboard shortcut
  const handleSaveRef = useRef(handleSave);
  handleSaveRef.current = handleSave;

  // Ctrl+S / Cmd+S
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        handleSaveRef.current();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  const title = doc?.title || docPath.split('/').pop() || docPath;

  return (
    <div className="flex flex-col h-full">
      {/* Header bar */}
      <div className="flex items-center gap-2 px-3 py-1.5 border-b border-border bg-background shrink-0">
        <FileText className="h-4 w-4 text-muted-foreground shrink-0" />
        <span className="text-sm font-medium truncate">{title}</span>
        {hasUnsavedChanges && (
          <span className="h-1.5 w-1.5 rounded-full bg-amber-500 shrink-0" />
        )}

        <div className="flex-1" />

        {/* Mode toggle */}
        <div className="flex rounded-md border border-border overflow-hidden">
          <button
            type="button"
            className={cn(
              'px-2 py-0.5 text-xs transition-colors',
              mode === 'edit'
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
            onClick={() => setMode('edit')}
          >
            Edit
          </button>
          <button
            type="button"
            className={cn(
              'px-2 py-0.5 text-xs transition-colors',
              mode === 'preview'
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
            onClick={() => setMode('preview')}
          >
            Preview
          </button>
        </div>

        {/* Save button */}
        {canWrite && (
          <button
            type="button"
            onClick={handleSave}
            disabled={!hasUnsavedChanges || isSaving}
            className={cn(
              'flex items-center gap-1 px-2 py-1 text-xs rounded-md transition-colors',
              hasUnsavedChanges
                ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                : 'text-muted-foreground cursor-not-allowed'
            )}
          >
            <Save className="h-3 w-3" />
            {isSaving ? 'Saving...' : 'Save'}
          </button>
        )}
      </div>

      {/* Editor / Preview */}
      <div className="flex-1 overflow-hidden min-h-0">
        {mode === 'edit' ? (
          <MarkdownEditor
            value={currentValue}
            onChange={(val) => setCurrentValue(val ?? '')}
            readOnly={!canWrite}
          />
        ) : (
          <div className="h-full overflow-y-auto p-6">
            <div className="doc-preview max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  code({ className: codeClassName, children }) {
                    if (codeClassName === 'language-mermaid') {
                      return <MermaidBlock code={String(children)} />;
                    }
                    return <code className={codeClassName}>{children}</code>;
                  },
                  pre({ children }) {
                    const child = children as React.ReactElement;
                    if (child?.type === MermaidBlock) {
                      return <>{children}</>;
                    }
                    return <pre>{children}</pre>;
                  },
                }}
              >
                {currentValue}
              </ReactMarkdown>
            </div>
          </div>
        )}
      </div>

      {/* Conflict dialog */}
      <DocExternalChangeDialog
        visible={conflict.hasConflict}
        onDiscard={() => resolveConflict('discard')}
        onIgnore={() => resolveConflict('ignore')}
      />
    </div>
  );
}

export default DocEditor;
