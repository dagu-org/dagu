import { components } from '@/api/v1/schema';
import { useCanWrite } from '@/contexts/AuthContext';
import { useDocTabContext } from '@/contexts/DocTabContext';
import { FileText, FilePlus } from 'lucide-react';
import React, { useCallback, useEffect, useState } from 'react';
import DocTreeNode, { type ContextAction } from './DocTreeNode';

type DocTreeNodeResponse = components['schemas']['DocTreeNodeResponse'];

type Props = {
  tree: DocTreeNodeResponse[] | undefined;
  onContextAction: (action: ContextAction) => void;
  onCreateNew: () => void;
  onSelectFile: (docPath: string, title: string) => void;
};

function collectAncestors(path: string): string[] {
  const parts = path.split('/');
  const ancestors: string[] = [];
  for (let i = 1; i < parts.length; i++) {
    ancestors.push(parts.slice(0, i).join('/'));
  }
  return ancestors;
}

function DocTreeSidebar({ tree, onContextAction, onCreateNew, onSelectFile }: Props) {
  const canWrite = useCanWrite();
  const { activeTabId, tabs } = useDocTabContext();
  const activeDocPath = activeTabId
    ? tabs.find(t => t.id === activeTabId)?.docPath || null
    : null;

  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // Auto-expand ancestors of active doc on initial load
  useEffect(() => {
    if (activeDocPath) {
      const ancestors = collectAncestors(activeDocPath);
      if (ancestors.length > 0) {
        setExpandedIds(prev => {
          const next = new Set(prev);
          let changed = false;
          for (const a of ancestors) {
            if (!next.has(a)) {
              next.add(a);
              changed = true;
            }
          }
          return changed ? next : prev;
        });
      }
    }
  }, [activeDocPath]);

  const handleToggleExpand = useCallback((id: string) => {
    setExpandedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const hasDocuments = tree && tree.length > 0;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Documents
        </span>
        {canWrite && (
          <button
            type="button"
            onClick={onCreateNew}
            className="p-1 rounded-sm hover:bg-accent text-muted-foreground hover:text-foreground"
            title="New Document"
          >
            <FilePlus className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Tree */}
      <div className="overflow-y-auto flex-1 py-1">
        {hasDocuments ? (
          tree.map((node) => (
            <DocTreeNode
              key={node.id}
              node={node}
              depth={0}
              expandedIds={expandedIds}
              activeDocPath={activeDocPath}
              onToggleExpand={handleToggleExpand}
              onSelectFile={onSelectFile}
              onContextAction={onContextAction}
              canWrite={canWrite}
            />
          ))
        ) : (
          <div className="flex flex-col items-center justify-center h-full gap-3 p-4 text-center">
            <FileText className="h-8 w-8 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">No documents yet.</p>
            {canWrite && (
              <button
                type="button"
                onClick={onCreateNew}
                className="text-sm text-primary hover:underline"
              >
                Create your first document
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default DocTreeSidebar;
