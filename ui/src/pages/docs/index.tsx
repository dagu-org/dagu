import SplitLayout from '@/components/SplitLayout';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { AppBarContext } from '@/contexts/AppBarContext';
import { DocTabProvider, useDocTabContext } from '@/contexts/DocTabContext';
import { UnsavedChangesProvider } from '@/contexts/UnsavedChangesContext';
import { useClient, useQuery } from '@/hooks/api';
import { useDocTreeSSE } from '@/hooks/useDocTreeSSE';
import { useIsMobile } from '@/hooks/useIsMobile';
import { ChevronLeft } from 'lucide-react';
import React, { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import ConfirmModal from '@/ui/ConfirmModal';
import { CreateDocModal } from './components/CreateDocModal';
import DocTabEditorPanel from './components/DocTabEditorPanel';
import DocTreeSidebar from './components/DocTreeSidebar';
import { RenameDocModal } from './components/RenameDocModal';
import type { ContextAction } from './components/DocTreeNode';

function titleFromPath(docPath: string): string {
  const segments = docPath.split('/');
  return segments[segments.length - 1] || docPath;
}

function DocsContent() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const navigate = useNavigate();
  const location = useLocation();
  const client = useClient();
  const { showToast } = useSimpleToast();
  const isMobile = useIsMobile();

  const {
    tabs,
    activeTabId,
    openDoc,
    closeTab,
    getActiveDocPath,
    updateTab,
    clearDraft,
    markTabSaved,
  } = useDocTabContext();

  // Mobile view state
  const [mobileView, setMobileView] = useState<'tree' | 'editor'>('tree');

  // Modal state
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [createParentDir, setCreateParentDir] = useState('');
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [renameModalOpen, setRenameModalOpen] = useState(false);
  const [renameDocPath, setRenameDocPath] = useState('');
  const [renameLoading, setRenameLoading] = useState(false);
  const [renameError, setRenameError] = useState<string | null>(null);

  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteDocPath, setDeleteDocPath] = useState('');
  const [deleteDocTitle, setDeleteDocTitle] = useState('');

  // Dual data source for tree
  const sseResult = useDocTreeSSE(true);
  const usePolling = sseResult.shouldUseFallback || !sseResult.isConnected;

  const { data: pollingData, mutate } = useQuery(
    '/docs',
    {
      params: {
        query: { remoteNode, perPage: 200 },
      },
    },
    {
      refreshInterval: usePolling ? 2000 : 0,
      keepPreviousData: true,
      isPaused: () => !usePolling,
    }
  );

  const treeData = sseResult.data ?? pollingData;

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('Documents');
  }, [appBarContext]);

  // URL ↔ Tab sync with loop prevention
  const isNavigatingRef = useRef(false);

  // URL → Tab
  useEffect(() => {
    if (isNavigatingRef.current) return;
    const docPath = location.pathname.replace(/^\/docs\/?/, '');
    if (docPath) {
      openDoc(decodeURIComponent(docPath), titleFromPath(decodeURIComponent(docPath)));
    }
  }, [location.pathname, openDoc]);

  // Tab → URL
  useEffect(() => {
    const docPath = getActiveDocPath();
    const currentPath = location.pathname.replace(/^\/docs\/?/, '');
    if (docPath && docPath !== decodeURIComponent(currentPath)) {
      isNavigatingRef.current = true;
      navigate('/docs/' + docPath, { replace: true });
      requestAnimationFrame(() => {
        isNavigatingRef.current = false;
      });
    } else if (!docPath && location.pathname !== '/docs') {
      isNavigatingRef.current = true;
      navigate('/docs', { replace: true });
      requestAnimationFrame(() => {
        isNavigatingRef.current = false;
      });
    }
  }, [activeTabId, getActiveDocPath, navigate, location.pathname]);

  // File selection handler
  const handleSelectFile = useCallback(
    (docPath: string, title: string) => {
      openDoc(docPath, title);
      if (isMobile) setMobileView('editor');
    },
    [openDoc, isMobile]
  );

  // Context menu actions
  const handleContextAction = useCallback(
    (action: ContextAction) => {
      switch (action.type) {
        case 'create':
          setCreateParentDir(action.parentDir);
          setCreateError(null);
          setCreateModalOpen(true);
          break;
        case 'rename':
          setRenameDocPath(action.docPath);
          setRenameError(null);
          setRenameModalOpen(true);
          break;
        case 'delete':
          setDeleteDocPath(action.docPath);
          setDeleteDocTitle(action.title);
          setDeleteConfirmOpen(true);
          break;
      }
    },
    []
  );

  // Create handler
  const handleCreate = useCallback(
    async (path: string) => {
      setCreateLoading(true);
      setCreateError(null);
      try {
        const { error } = await client.POST('/docs', {
          params: { query: { remoteNode } },
          body: { id: path, content: '' },
        });
        if (error) {
          setCreateError((error as { message?: string }).message || 'Failed to create document');
          return;
        }
        mutate();
        openDoc(path, titleFromPath(path));
        showToast('Document created');
        setCreateModalOpen(false);
      } catch {
        setCreateError('Failed to create document');
      } finally {
        setCreateLoading(false);
      }
    },
    [client, remoteNode, mutate, openDoc, showToast]
  );

  // Rename handler
  const handleRename = useCallback(
    async (newPath: string) => {
      setRenameLoading(true);
      setRenameError(null);
      try {
        const { error } = await client.POST('/docs/doc/rename', {
          params: { query: { remoteNode, path: renameDocPath } },
          body: { newPath },
        });
        if (error) {
          setRenameError((error as { message?: string }).message || 'Failed to rename document');
          return;
        }
        mutate();
        // Update tab if this doc is open
        const openTab = tabs.find((t) => t.docPath === renameDocPath);
        if (openTab) {
          updateTab(openTab.id, { docPath: newPath, title: titleFromPath(newPath) });
        }
        showToast('Document renamed');
        setRenameModalOpen(false);
      } catch {
        setRenameError('Failed to rename document');
      } finally {
        setRenameLoading(false);
      }
    },
    [client, remoteNode, renameDocPath, mutate, tabs, updateTab, showToast]
  );

  // Delete handler
  const handleDelete = useCallback(async () => {
    try {
      const { error } = await client.DELETE('/docs/doc', {
        params: { query: { remoteNode, path: deleteDocPath } },
      });
      if (error) {
        showToast('Failed to delete document');
        return;
      }
      mutate();
      // Close tab if open
      const openTab = tabs.find((t) => t.docPath === deleteDocPath);
      if (openTab) {
        clearDraft(openTab.id);
        markTabSaved(openTab.id);
        closeTab(openTab.id);
      }
      showToast('Document deleted');
    } catch {
      showToast('Failed to delete document');
    } finally {
      setDeleteConfirmOpen(false);
    }
  }, [client, remoteNode, deleteDocPath, mutate, tabs, closeTab, clearDraft, markTabSaved, showToast]);

  const leftPanel = (
    <DocTreeSidebar
      tree={treeData?.tree}
      onContextAction={handleContextAction}
      onCreateNew={() => {
        setCreateParentDir('');
        setCreateError(null);
        setCreateModalOpen(true);
      }}
      onSelectFile={handleSelectFile}
    />
  );

  const rightPanel =
    tabs.length > 0 ? <DocTabEditorPanel /> : null;

  // Mobile layout
  if (isMobile) {
    return (
      <div className="-m-4 w-[calc(100%+2rem)] h-[calc(100%+2rem)]">
        {mobileView === 'tree' ? (
          <div className="h-full">{leftPanel}</div>
        ) : (
          <div className="flex flex-col h-full">
            <button
              type="button"
              className="flex items-center gap-1 px-3 py-2 text-sm text-muted-foreground hover:text-foreground border-b border-border"
              onClick={() => setMobileView('tree')}
            >
              <ChevronLeft className="h-4 w-4" />
              Documents
            </button>
            <div className="flex-1 overflow-hidden min-h-0">
              {rightPanel || (
                <div className="flex items-center justify-center h-full">
                  <p className="text-sm text-muted-foreground">
                    Select a document to start editing.
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Modals */}
        <CreateDocModal
          isOpen={createModalOpen}
          onClose={() => setCreateModalOpen(false)}
          onSubmit={handleCreate}
          parentDir={createParentDir}
          isLoading={createLoading}
          externalError={createError}
        />
        <RenameDocModal
          isOpen={renameModalOpen}
          onClose={() => setRenameModalOpen(false)}
          onSubmit={handleRename}
          currentPath={renameDocPath}
          isLoading={renameLoading}
          externalError={renameError}
        />
        <ConfirmModal
          title="Delete Document"
          buttonText="Delete"
          visible={deleteConfirmOpen}
          dismissModal={() => setDeleteConfirmOpen(false)}
          onSubmit={handleDelete}
        >
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <strong>{deleteDocTitle}</strong>? This
            action cannot be undone.
          </p>
        </ConfirmModal>
      </div>
    );
  }

  // Desktop layout
  return (
    <div className="-m-4 md:-m-6 w-[calc(100%+2rem)] md:w-[calc(100%+3rem)] h-[calc(100%+2rem)] md:h-[calc(100%+3rem)]">
      <SplitLayout
        leftPanel={leftPanel}
        rightPanel={rightPanel}
        defaultLeftWidth={25}
        minLeftWidth={15}
        maxLeftWidth={40}
        storageKey="docTreeWidth"
        emptyRightMessage="Select a document to start editing"
      />

      {/* Modals */}
      <CreateDocModal
        isOpen={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onSubmit={handleCreate}
        parentDir={createParentDir}
        isLoading={createLoading}
        externalError={createError}
      />
      <RenameDocModal
        isOpen={renameModalOpen}
        onClose={() => setRenameModalOpen(false)}
        onSubmit={handleRename}
        currentPath={renameDocPath}
        isLoading={renameLoading}
        externalError={renameError}
      />
      <ConfirmModal
        title="Delete Document"
        buttonText="Delete"
        visible={deleteConfirmOpen}
        dismissModal={() => setDeleteConfirmOpen(false)}
        onSubmit={handleDelete}
      >
        <p className="text-sm text-muted-foreground">
          Are you sure you want to delete <strong>{deleteDocTitle}</strong>? This
          action cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}

function DocsPage() {
  return (
    <UnsavedChangesProvider>
      <DocTabProvider>
        <DocsContent />
      </DocTabProvider>
    </UnsavedChangesProvider>
  );
}

export default DocsPage;
