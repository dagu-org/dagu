import { useCanWrite } from '@/contexts/AuthContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { cn } from '@/lib/utils';
import { FileText, FilePlus, Loader2 } from 'lucide-react';
import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { components } from '@/api/v1/schema';
import { DocTreeNode } from './DocTreeNode';
import { CreateDocModal } from './CreateDocModal';
import { RenameDocModal } from './RenameDocModal';
import { DeleteDocDialog } from './DeleteDocDialog';

type DocTreeNodeResponse = components['schemas']['DocTreeNodeResponse'];

interface DocTreeSidebarProps {
  activeDocId: string | null;
  onSelectDoc: (id: string, title: string) => void;
  onDocCreated?: (id: string) => void;
  onDocDeleted?: (id: string) => void;
  onDocRenamed?: (oldId: string, newId: string) => void;
}

export function DocTreeSidebar({
  activeDocId,
  onSelectDoc,
  onDocCreated,
  onDocDeleted,
  onDocRenamed,
}: DocTreeSidebarProps) {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const canWrite = useCanWrite();

  const [tree, setTree] = useState<DocTreeNodeResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());

  // CRUD dialog state
  const [createModal, setCreateModal] = useState<{ open: boolean; parentDir?: string }>({ open: false });
  const [renameModal, setRenameModal] = useState<{ open: boolean; docPath: string }>({ open: false, docPath: '' });
  const [deleteDialog, setDeleteDialog] = useState<{ open: boolean; docPath: string }>({ open: false, docPath: '' });

  const fetchTree = useCallback(async () => {
    try {
      const { data } = await client.GET('/docs', {
        params: { query: { remoteNode } },
      });
      if (data?.tree) {
        setTree(data.tree);
      }
    } catch {
      // Best-effort
    } finally {
      setLoading(false);
    }
  }, [client, remoteNode]);

  useEffect(() => {
    fetchTree();
  }, [fetchTree]);

  // Auto-expand path to active doc
  useEffect(() => {
    if (!activeDocId) return;
    const parts = activeDocId.split('/');
    const newExpanded = new Set(expandedDirs);
    let path = '';
    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i] as string;
      path = path ? `${path}/${part}` : part;
      newExpanded.add(path);
    }
    setExpandedDirs(newExpanded);
  }, [activeDocId]);

  const toggleDir = useCallback((id: string) => {
    setExpandedDirs(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const handleCreate = useCallback(async (docPath: string) => {
    try {
      const { error } = await client.POST('/docs', {
        params: { query: { remoteNode } },
        body: { id: docPath, content: '' },
      });
      if (error) return;
      setCreateModal({ open: false });
      await fetchTree();
      onDocCreated?.(docPath);
    } catch {
      // Best-effort
    }
  }, [client, remoteNode, fetchTree, onDocCreated]);

  const handleRename = useCallback(async (newPath: string) => {
    const oldPath = renameModal.docPath;
    try {
      const { error } = await client.POST('/docs/{docPath}/rename', {
        params: { path: { docPath: oldPath }, query: { remoteNode } },
        body: { newPath },
      });
      if (error) return;
      setRenameModal({ open: false, docPath: '' });
      await fetchTree();
      onDocRenamed?.(oldPath, newPath);
    } catch {
      // Best-effort
    }
  }, [client, remoteNode, renameModal.docPath, fetchTree, onDocRenamed]);

  const handleDelete = useCallback(async () => {
    const docPath = deleteDialog.docPath;
    try {
      const { error } = await client.DELETE('/docs/{docPath}', {
        params: { path: { docPath }, query: { remoteNode } },
      });
      if (error) return;
      setDeleteDialog({ open: false, docPath: '' });
      await fetchTree();
      onDocDeleted?.(docPath);
    } catch {
      // Best-effort
    }
  }, [client, remoteNode, deleteDialog.docPath, fetchTree, onDocDeleted]);

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border shrink-0">
        <span className="text-sm font-medium text-foreground">Documents</span>
        {canWrite && (
          <button
            onClick={() => setCreateModal({ open: true })}
            className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground"
            title="New document"
          >
            <FilePlus className="w-4 h-4" />
          </button>
        )}
      </div>

      {/* Tree */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden py-1">
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
          </div>
        ) : tree.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 gap-2 text-muted-foreground">
            <FileText className="w-8 h-8" />
            <p className="text-sm">No documents yet.</p>
            {canWrite && (
              <button
                onClick={() => setCreateModal({ open: true })}
                className="text-xs text-primary hover:underline"
              >
                Create one
              </button>
            )}
          </div>
        ) : (
          tree.map(node => (
            <DocTreeNode
              key={node.id}
              node={node}
              depth={0}
              activeDocId={activeDocId}
              expandedDirs={expandedDirs}
              onToggleDir={toggleDir}
              onSelectDoc={onSelectDoc}
              onCreateDoc={canWrite ? (parentDir) => setCreateModal({ open: true, parentDir }) : undefined}
              onRenameDoc={canWrite ? (id) => setRenameModal({ open: true, docPath: id }) : undefined}
              onDeleteDoc={canWrite ? (id) => setDeleteDialog({ open: true, docPath: id }) : undefined}
              canWrite={canWrite}
            />
          ))
        )}
      </div>

      {/* Modals */}
      <CreateDocModal
        visible={createModal.open}
        parentDir={createModal.parentDir}
        onSubmit={handleCreate}
        onDismiss={() => setCreateModal({ open: false })}
      />
      <RenameDocModal
        visible={renameModal.open}
        currentPath={renameModal.docPath}
        onSubmit={handleRename}
        onDismiss={() => setRenameModal({ open: false, docPath: '' })}
      />
      <DeleteDocDialog
        visible={deleteDialog.open}
        docPath={deleteDialog.docPath}
        onConfirm={handleDelete}
        onDismiss={() => setDeleteDialog({ open: false, docPath: '' })}
      />
    </div>
  );
}
