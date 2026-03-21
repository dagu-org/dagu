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
import {
  liveFallbackOptions,
  useLiveConnection,
  useLiveDoc,
} from '@/hooks/useAppLive';
import { useContentEditor } from '@/hooks/useContentEditor';
import { cn } from '@/lib/utils';
import { slugifyHeading } from '@/lib/text-utils';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Check, ClipboardCopy, Copy, FileText, Save, Trash2, Undo2 } from 'lucide-react';
import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import DocExternalChangeDialog from './DocExternalChangeDialog';

function headingId(children: React.ReactNode): string {
  const text = typeof children === 'string'
    ? children
    : Array.isArray(children)
      ? children.map((c) => (typeof c === 'string' ? c : '')).join('')
      : String(children ?? '');
  return slugifyHeading(text);
}

type Props = {
  tabId: string;
  docPath: string;
  onDeleteDoc?: () => void;
  onContentChange?: (content: string | null) => void;
};

function DocEditor({ tabId, docPath, onDeleteDoc, onContentChange }: Props) {
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

  const liveState = useLiveConnection();

  // Fetch doc — SWR is the single source of truth, refreshed by live invalidations
  const { data: doc, mutate: mutateDoc } = useQuery(
    '/docs/doc',
    {
      params: {
        query: {
          remoteNode,
          path: docPath,
        },
      },
    },
    liveFallbackOptions(liveState)
  );
  useLiveDoc(docPath, mutateDoc);
  const serverContent = doc?.content ?? null;

  // Change tracking (source-agnostic)
  const {
    currentValue,
    setCurrentValue,
    hasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
    discardChanges,
  } = useContentEditor({
    key: `${docPath}:${remoteNode}`,
    serverContent,
  });

  const [mode, setMode] = useState<'edit' | 'preview'>(() => {
    const stored = localStorage.getItem('doc-editor-mode');
    return stored === 'preview' ? 'preview' : 'edit';
  });
  const [isSaving, setIsSaving] = useState(false);

  // Use refs for cleanup and to avoid stale closures / unnecessary callback recreation
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
        setDraft(tabId, currentValueRef.current ?? '');
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

  // Report content changes to parent for outline panel (debounced)
  const contentChangeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (contentChangeTimerRef.current) {
      clearTimeout(contentChangeTimerRef.current);
    }
    contentChangeTimerRef.current = setTimeout(() => {
      onContentChange?.(currentValue);
    }, 300);
    return () => {
      if (contentChangeTimerRef.current) clearTimeout(contentChangeTimerRef.current);
    };
  }, [currentValue, onContentChange]);

  const handleSave = useCallback(async () => {
    if (isSaving || !hasUnsavedChangesRef.current) return;
    setIsSaving(true);
    try {
      const { error } = await client.PATCH('/docs/doc', {
        params: { query: { remoteNode, path: docPath } },
        body: { content: currentValueRef.current ?? '' },
      });
      if (error) {
        showToast('Failed to save document');
      } else {
        markAsSaved(currentValueRef.current ?? '');
        // Revalidate SWR cache from server as safety net
        mutateDoc();
        markTabSaved(tabId);
        clearDraft(tabId);
        showToast('Document saved');
      }
    } catch {
      showToast('Failed to save document');
    } finally {
      setIsSaving(false);
    }
  }, [isSaving, client, remoteNode, docPath, markAsSaved, mutateDoc, markTabSaved, clearDraft, tabId, showToast]);

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

  const [copiedPath, setCopiedPath] = useState(false);
  const [copiedContent, setCopiedContent] = useState(false);

  const copyFilePath = useCallback(async () => {
    const fp = doc?.filePath;
    if (!fp) return;
    try {
      await navigator.clipboard.writeText(fp);
      setCopiedPath(true);
      setTimeout(() => setCopiedPath(false), 2000);
    } catch {
      const textArea = document.createElement('textarea');
      textArea.value = fp;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopiedPath(true);
      setTimeout(() => setCopiedPath(false), 2000);
    }
  }, [doc?.filePath]);

  const copyContent = useCallback(async () => {
    const text = currentValue ?? '';
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopiedContent(true);
      setTimeout(() => setCopiedContent(false), 2000);
    } catch {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopiedContent(true);
      setTimeout(() => setCopiedContent(false), 2000);
    }
  }, [currentValue]);

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

        {doc?.filePath && (
          <button
            type="button"
            onClick={copyFilePath}
            className="inline-flex items-center gap-1 px-1.5 py-0.5 text-xs rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-all shrink-0"
            title={`Copy file path: ${doc.filePath}`}
          >
            {copiedPath ? (
              <Check className="h-3.5 w-3.5 text-green-500" />
            ) : (
              <Copy className="h-3.5 w-3.5" />
            )}
          </button>
        )}

        <div className="flex-1" />

        {/* Copy content */}
        <button
          type="button"
          onClick={copyContent}
          disabled={!currentValue}
          className="flex items-center gap-1 px-2 py-0.5 text-xs rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          title="Copy content"
        >
          {copiedContent ? (
            <Check className="h-3 w-3 text-green-500" />
          ) : (
            <ClipboardCopy className="h-3 w-3" />
          )}
          <span>Copy</span>
        </button>

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

        {/* Discard button */}
        {canWrite && hasUnsavedChanges && (
          <button
            type="button"
            onClick={() => {
              discardChanges();
              clearDraft(tabId);
              markTabSaved(tabId);
            }}
            className="flex items-center gap-1 px-2 py-1 text-xs rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
            title="Discard changes"
          >
            <Undo2 className="h-3 w-3" />
            Discard
          </button>
        )}

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
        {canWrite && onDeleteDoc && (
          <button
            type="button"
            onClick={onDeleteDoc}
            className="flex items-center gap-1 px-2 py-1 text-xs rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
            title="Delete document"
            aria-label="Delete document"
          >
            <Trash2 className="h-3 w-3" />
          </button>
        )}
      </div>

      {/* Editor / Preview */}
      <div className="flex-1 overflow-hidden min-h-0">
        {mode === 'edit' ? (
          <MarkdownEditor
            value={currentValue ?? ''}
            onChange={(val) => setCurrentValue(val ?? '')}
            readOnly={!canWrite}
          />
        ) : (
          <div className="h-full overflow-y-auto p-6">
            <div className="doc-preview max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  h1: ({ children }) => <h1 id={headingId(children)}>{children}</h1>,
                  h2: ({ children }) => <h2 id={headingId(children)}>{children}</h2>,
                  h3: ({ children }) => <h3 id={headingId(children)}>{children}</h3>,
                  h4: ({ children }) => <h4 id={headingId(children)}>{children}</h4>,
                  h5: ({ children }) => <h5 id={headingId(children)}>{children}</h5>,
                  h6: ({ children }) => <h6 id={headingId(children)}>{children}</h6>,
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
