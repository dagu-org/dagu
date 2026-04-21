// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { ChevronLeft } from 'lucide-react';
import React, {
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  PathsDocsGetParametersQueryOrder,
  PathsDocsGetParametersQuerySort,
  WorkspaceMutationScope,
} from '@/api/v1/schema';
import SplitLayout from '@/components/SplitLayout';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useCanWrite } from '@/contexts/AuthContext';
import { DocTabProvider, useDocTabContext } from '@/contexts/DocTabContext';
import { usePageContext } from '@/contexts/PageContext';
import { UnsavedChangesProvider } from '@/contexts/UnsavedChangesContext';
import { useUserPreferences } from '@/contexts/UserPreference';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { useClient, useQuery } from '@/hooks/api';
import { useIsMobile } from '@/hooks/useIsMobile';
import { useDocTreeSSE } from '@/hooks/useDocTreeSSE';
import { sseFallbackOptions, useSSECacheSync } from '@/hooks/useSSECacheSync';
import {
  DEFAULT_WORKSPACE_DISPLAY_NAME,
  isMutableWorkspaceSelection,
  sanitizeWorkspaceName,
  workspaceMutationSelectionQuery,
  workspaceSelectionKey,
  workspaceSelectionQuery,
  visibleDocumentPathForWorkspace,
} from '@/lib/workspace';
import ConfirmModal from '@/ui/ConfirmModal';
import { CreateDocModal } from './components/CreateDocModal';
import DocTabEditorPanel from './components/DocTabEditorPanel';
import DocTreeSidebar from './components/DocTreeSidebar';
import { RenameDocModal } from './components/RenameDocModal';
import type { ContextAction } from './components/DocArboristNode';

function titleFromPath(docPath: string): string {
  const segments = docPath.split('/');
  return segments[segments.length - 1] || docPath;
}

function encodeDocPathForURL(docPath: string): string {
  return docPath.split('/').map(encodeURIComponent).join('/');
}

function workspaceSearchForDocTab(workspace?: string | null): string {
  const sanitized = sanitizeWorkspaceName(workspace ?? '');
  if (sanitized) {
    return `?workspace=${encodeURIComponent(sanitized)}`;
  }
  return `?workspaceScope=${WorkspaceMutationScope.default}`;
}

function DocsContent() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const navigate = useNavigate();
  const location = useLocation();
  const client = useClient();
  const { showToast } = useSimpleToast();
  const isMobile = useIsMobile();

  const { selectedTemplate, selectTemplate } = useCockpitState();
  const selectedWorkspace = appBarContext.selectedWorkspace || '';
  const workspaceSelection = appBarContext.workspaceSelection;
  const workspaceQuery = React.useMemo(
    () => workspaceSelectionQuery(workspaceSelection),
    [workspaceSelection]
  );
  const workspaceMutationQuery = React.useMemo(
    () => workspaceMutationSelectionQuery(workspaceSelection),
    [workspaceSelection]
  );
  const canWrite = useCanWrite();
  const canMutateDocs =
    canWrite && isMutableWorkspaceSelection(workspaceSelection);

  const { setContext } = usePageContext();
  const {
    tabs,
    activeTabId,
    openDoc,
    closeTab,
    updateTab,
    clearDraft,
    markTabSaved,
  } = useDocTabContext();

  // Mobile view state
  const [mobileView, setMobileView] = useState<'tree' | 'editor'>('tree');

  // Active doc content for outline panel
  const [activeDocContent, setActiveDocContent] = useState<string | null>(null);

  // Clear stale content when switching tabs so the outline panel doesn't show old headings
  useEffect(() => {
    setActiveDocContent(null);
  }, [activeTabId]);

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

  // Batch delete state
  const [batchDeletePaths, setBatchDeletePaths] = useState<string[]>([]);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);

  // Sort preferences
  const { preferences, updatePreference } = useUserPreferences();
  const { docSortField, docSortOrder } = preferences;
  const sort = docSortField as PathsDocsGetParametersQuerySort;
  const order = docSortOrder as PathsDocsGetParametersQueryOrder;

  const docTreeSSE = useDocTreeSSE({
    sort,
    order,
    remoteNode,
    ...workspaceQuery,
  });

  const {
    data: treeData,
    mutate,
    error: treeError,
    isLoading: treeIsLoading,
  } = useQuery(
    '/docs',
    {
      params: {
        query: {
          remoteNode,
          perPage: 200,
          sort,
          order,
          ...workspaceQuery,
        },
      },
    },
    {
      ...sseFallbackOptions(docTreeSSE),
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      keepPreviousData: true,
    }
  );
  useSSECacheSync(docTreeSSE, mutate);

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('Documents');
  }, [appBarContext]);

  // Set page context for agent chat (mirrors DAG detail page pattern)
  useEffect(() => {
    const activeTab = tabs.find((t) => t.id === activeTabId);
    if (activeTab) {
      setContext({
        docPath: activeTab.docPath,
        docTitle: activeTab.title,
        source: 'docs-page',
      });
    } else {
      setContext(null);
    }
    return () => {
      setContext(null);
    };
  }, [activeTabId, tabs, setContext]);

  // URL ↔ Tab sync with loop prevention
  const isNavigatingRef = useRef(false);
  const isInitialMountRef = useRef(true);

  // URL → Tab (source of truth on mount)
  useEffect(() => {
    if (isNavigatingRef.current) return;
    const docPath = location.pathname.replace(/^\/docs\/?/, '');
    if (docPath) {
      const searchParams = new URLSearchParams(location.search);
      const queryWorkspace = sanitizeWorkspaceName(
        searchParams.get('workspace') ?? ''
      );
      const isNoWorkspaceURL =
        searchParams.get('workspaceScope') === WorkspaceMutationScope.default;
      const docWorkspace = isNoWorkspaceURL
        ? null
        : queryWorkspace || selectedWorkspace || null;
      const decodedPath = decodeURIComponent(docPath);
      openDoc(decodedPath, titleFromPath(decodedPath), docWorkspace);
    }
  }, [location.pathname, location.search, openDoc, selectedWorkspace]);

  // Tab → URL (skip on initial mount — URL takes precedence)
  useEffect(() => {
    if (isInitialMountRef.current) {
      isInitialMountRef.current = false;
      return;
    }
    if (isNavigatingRef.current) return;
    const activeTab = activeTabId
      ? tabs.find((t) => t.id === activeTabId)
      : null;
    const docPath = activeTab?.docPath;
    const currentPath = location.pathname.replace(/^\/docs\/?/, '');
    const targetSearch = activeTab
      ? workspaceSearchForDocTab(activeTab.workspace)
      : '';
    const encodedDocPath = docPath ? encodeDocPathForURL(docPath) : '';
    if (docPath && encodedDocPath !== currentPath) {
      isNavigatingRef.current = true;
      navigate(`/docs/${encodedDocPath}${targetSearch}`, { replace: true });
      requestAnimationFrame(() => {
        isNavigatingRef.current = false;
      });
    } else if (docPath && location.search !== targetSearch) {
      isNavigatingRef.current = true;
      navigate(`/docs/${encodedDocPath}${targetSearch}`, { replace: true });
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
  }, [activeTabId, tabs, navigate, location.pathname, location.search]);

  // File selection handler
  const handleSelectFile = useCallback(
    (docPath: string, title: string, workspace?: string | null) => {
      const visiblePath = visibleDocumentPathForWorkspace(docPath, workspace);
      openDoc(visiblePath, title, workspace ?? null);
      if (isMobile) setMobileView('editor');
    },
    [openDoc, isMobile]
  );

  // Track selected IDs from sidebar for batch operations
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const handleSelectionChange = useCallback((ids: string[]) => {
    setSelectedIds(ids);
  }, []);

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
        case 'deleteBatch':
          setBatchDeletePaths([...selectedIds]);
          setBatchDeleteConfirmOpen(true);
          break;
      }
    },
    [selectedIds]
  );

  // Create handler
  const handleCreate = useCallback(
    async (path: string) => {
      if (!canMutateDocs || !workspaceMutationQuery) {
        setCreateError(
          `Select a workspace or ${DEFAULT_WORKSPACE_DISPLAY_NAME} before creating.`
        );
        return;
      }
      setCreateLoading(true);
      setCreateError(null);
      try {
        const { error } = await client.POST('/docs', {
          params: { query: { remoteNode, ...workspaceMutationQuery } },
          body: { id: path, content: '' },
        });
        if (error) {
          setCreateError(error?.message || 'Failed to create document');
          return;
        }
        mutate();
        const createdWorkspace =
          workspaceMutationQuery.workspaceScope ===
          WorkspaceMutationScope.workspace
            ? (workspaceMutationQuery.workspace ?? null)
            : null;
        openDoc(path, titleFromPath(path), createdWorkspace);
        showToast('Document created');
        setCreateModalOpen(false);
      } catch {
        setCreateError('Failed to create document');
      } finally {
        setCreateLoading(false);
      }
    },
    [
      canMutateDocs,
      client,
      remoteNode,
      workspaceMutationQuery,
      mutate,
      openDoc,
      showToast,
    ]
  );

  // Rename handler (from modal)
  const handleRenameModal = useCallback(
    async (newPath: string) => {
      if (!canMutateDocs || !workspaceMutationQuery) {
        setRenameError(
          `Select a workspace or ${DEFAULT_WORKSPACE_DISPLAY_NAME} before renaming.`
        );
        return;
      }
      setRenameLoading(true);
      setRenameError(null);
      try {
        const { error } = await client.POST('/docs/doc/rename', {
          params: {
            query: {
              remoteNode,
              path: renameDocPath,
              ...workspaceMutationQuery,
            },
          },
          body: { newPath },
        });
        if (error) {
          setRenameError(error?.message || 'Failed to rename document');
          return;
        }
        mutate();
        // Update all tabs under the renamed path (handles both file and directory renames).
        for (const tab of tabs) {
          if (
            tab.docPath === renameDocPath ||
            tab.docPath.startsWith(renameDocPath + '/')
          ) {
            const updatedPath =
              newPath + tab.docPath.slice(renameDocPath.length);
            updateTab(tab.id, {
              docPath: updatedPath,
              title: titleFromPath(updatedPath),
            });
          }
        }
        showToast('Document renamed');
        setRenameModalOpen(false);
      } catch {
        setRenameError('Failed to rename document');
      } finally {
        setRenameLoading(false);
      }
    },
    [
      client,
      canMutateDocs,
      remoteNode,
      workspaceMutationQuery,
      renameDocPath,
      mutate,
      tabs,
      updateTab,
      showToast,
    ]
  );

  // Shared path-change handler for rename and move
  const handlePathChange = useCallback(
    async (oldPath: string, newPath: string, action: 'renamed' | 'moved') => {
      if (!canMutateDocs || !workspaceMutationQuery) {
        showToast(
          `Select a workspace or ${DEFAULT_WORKSPACE_DISPLAY_NAME} before editing documents`
        );
        return;
      }
      try {
        const { error } = await client.POST('/docs/doc/rename', {
          params: {
            query: { remoteNode, path: oldPath, ...workspaceMutationQuery },
          },
          body: { newPath },
        });
        if (error) {
          showToast(
            error?.message ||
              `Failed to ${action === 'renamed' ? 'rename' : 'move'} document`
          );
          mutate();
          return;
        }
        mutate();
        // Update ALL tabs under the moved path (handles both file and directory moves).
        for (const tab of tabs) {
          if (
            tab.docPath === oldPath ||
            tab.docPath.startsWith(oldPath + '/')
          ) {
            const updatedPath = newPath + tab.docPath.slice(oldPath.length);
            updateTab(tab.id, {
              docPath: updatedPath,
              title: titleFromPath(updatedPath),
            });
          }
        }
        showToast(`Document ${action}`);
      } catch {
        showToast(
          `Failed to ${action === 'renamed' ? 'rename' : 'move'} document`
        );
        mutate();
      }
    },
    [
      canMutateDocs,
      client,
      remoteNode,
      workspaceMutationQuery,
      mutate,
      tabs,
      updateTab,
      showToast,
    ]
  );

  const handleInlineRename = useCallback(
    (oldPath: string, newPath: string) =>
      handlePathChange(oldPath, newPath, 'renamed'),
    [handlePathChange]
  );

  const handleMove = useCallback(
    (oldPath: string, newPath: string) =>
      handlePathChange(oldPath, newPath, 'moved'),
    [handlePathChange]
  );

  // Heading click for outline panel
  const handleHeadingClick = useCallback((anchor: string) => {
    // Find the heading in the preview panel and scroll to it
    const el = document.getElementById(anchor);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, []);

  // Delete handler (supports both files and directories)
  const handleDelete = useCallback(async () => {
    if (!canMutateDocs || !workspaceMutationQuery) {
      showToast(
        `Select a workspace or ${DEFAULT_WORKSPACE_DISPLAY_NAME} before deleting documents`
      );
      setDeleteConfirmOpen(false);
      return;
    }
    try {
      const { error } = await client.DELETE('/docs/doc', {
        params: {
          query: { remoteNode, path: deleteDocPath, ...workspaceMutationQuery },
        },
      });
      if (error) {
        showToast('Failed to delete document');
        return;
      }
      mutate();
      // Close tabs for deleted path (exact match + prefix for directories)
      for (const tab of tabs) {
        if (
          tab.docPath === deleteDocPath ||
          tab.docPath.startsWith(deleteDocPath + '/')
        ) {
          clearDraft(tab.id);
          markTabSaved(tab.id);
          closeTab(tab.id);
        }
      }
      showToast('Document deleted');
    } catch {
      showToast('Failed to delete document');
    } finally {
      setDeleteConfirmOpen(false);
    }
  }, [
    client,
    canMutateDocs,
    remoteNode,
    workspaceMutationQuery,
    deleteDocPath,
    mutate,
    tabs,
    closeTab,
    clearDraft,
    markTabSaved,
    showToast,
  ]);

  // Batch delete handler
  const handleBatchDelete = useCallback(async () => {
    if (!canMutateDocs || !workspaceMutationQuery) {
      showToast(
        `Select a workspace or ${DEFAULT_WORKSPACE_DISPLAY_NAME} before deleting documents`
      );
      setBatchDeleteConfirmOpen(false);
      setBatchDeletePaths([]);
      return;
    }
    try {
      const { data, error } = await client.POST('/docs/delete-batch', {
        params: { query: { remoteNode, ...workspaceMutationQuery } },
        body: { paths: batchDeletePaths },
      });
      if (error) {
        showToast('Failed to delete documents');
        return;
      }
      mutate();
      // Close tabs for all deleted paths (exact + prefix for directories)
      const deletedSet = new Set(data.deleted);
      for (const tab of tabs) {
        const shouldClose =
          deletedSet.has(tab.docPath) ||
          [...deletedSet].some((dp) => tab.docPath.startsWith(dp + '/'));
        if (shouldClose) {
          clearDraft(tab.id);
          markTabSaved(tab.id);
          closeTab(tab.id);
        }
      }
      const failCount = data.failed?.length || 0;
      if (failCount > 0) {
        showToast(`Deleted ${data.deleted.length}, ${failCount} failed`);
      } else {
        showToast(`Deleted ${data.deleted.length} items`);
      }
    } catch {
      showToast('Failed to delete documents');
    } finally {
      setBatchDeleteConfirmOpen(false);
      setBatchDeletePaths([]);
    }
  }, [
    batchDeletePaths,
    canMutateDocs,
    client,
    remoteNode,
    workspaceMutationQuery,
    mutate,
    tabs,
    closeTab,
    clearDraft,
    markTabSaved,
    showToast,
  ]);

  // Batch delete from selection bar
  const handleBatchDeleteFromBar = useCallback((paths: string[]) => {
    setBatchDeletePaths(paths);
    setBatchDeleteConfirmOpen(true);
  }, []);

  // Delete triggered from tab menu or editor header
  const handleDeleteFromTab = useCallback((docPath: string, title: string) => {
    setDeleteDocPath(docPath);
    setDeleteDocTitle(title);
    setDeleteConfirmOpen(true);
  }, []);

  const leftPanel = (
    <DocTreeSidebar
      tree={treeData?.tree}
      isLoading={treeIsLoading}
      error={treeError}
      onRetry={() => mutate()}
      onContextAction={handleContextAction}
      onCreateNew={() => {
        setCreateParentDir('');
        setCreateError(null);
        setCreateModalOpen(true);
      }}
      onSelectFile={handleSelectFile}
      onRename={handleInlineRename}
      onMove={handleMove}
      onBatchDelete={handleBatchDeleteFromBar}
      onSelectionChange={handleSelectionChange}
      canMutate={canMutateDocs}
      activeDocContent={activeDocContent}
      onHeadingClick={handleHeadingClick}
      sortField={docSortField}
      sortOrder={docSortOrder}
      onSortChange={(field, order) => {
        updatePreference('docSortField', field);
        updatePreference('docSortOrder', order);
      }}
    />
  );

  const cockpitToolbar = (
    <div className="[&>div]:mb-0">
      <CockpitToolbar
        selectedWorkspace={selectedWorkspace}
        selectedTemplate={selectedTemplate}
        onSelectTemplate={selectTemplate}
      />
    </div>
  );

  const rightPanel =
    tabs.length > 0 ? (
      <DocTabEditorPanel
        onDeleteDoc={handleDeleteFromTab}
        toolbar={cockpitToolbar}
        onContentChange={setActiveDocContent}
      />
    ) : null;

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
          onSubmit={handleRenameModal}
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
            Are you sure you want to delete <strong>{deleteDocTitle}</strong>?
            This action cannot be undone.
          </p>
        </ConfirmModal>
        <ConfirmModal
          title="Delete Documents"
          buttonText={`Delete ${batchDeletePaths.length} items`}
          visible={batchDeleteConfirmOpen}
          dismissModal={() => setBatchDeleteConfirmOpen(false)}
          onSubmit={handleBatchDelete}
        >
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete {batchDeletePaths.length} items?
            This cannot be undone.
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
        onSubmit={handleRenameModal}
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
          Are you sure you want to delete <strong>{deleteDocTitle}</strong>?
          This action cannot be undone.
        </p>
      </ConfirmModal>
      <ConfirmModal
        title="Delete Documents"
        buttonText={`Delete ${batchDeletePaths.length} items`}
        visible={batchDeleteConfirmOpen}
        dismissModal={() => setBatchDeleteConfirmOpen(false)}
        onSubmit={handleBatchDelete}
      >
        <p className="text-sm text-muted-foreground">
          Are you sure you want to delete {batchDeletePaths.length} items? This
          cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}

function DocsPage() {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const docTabStorageKey = `dagu_doc_tabs:${JSON.stringify({
    remoteNode,
    workspace: workspaceSelectionKey(appBarContext.workspaceSelection),
  })}`;

  return (
    <UnsavedChangesProvider>
      <DocTabProvider key={docTabStorageKey} storageKey={docTabStorageKey}>
        <DocsContent />
      </DocTabProvider>
    </UnsavedChangesProvider>
  );
}

export default DocsPage;
