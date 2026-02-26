import SplitLayout from '@/components/SplitLayout';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useCallback, useContext, useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { DocTabProvider, useDocTabContext } from './contexts/DocTabContext';
import { DocTreeSidebar } from './components/DocTreeSidebar';
import { DocTabEditorPanel } from './components/DocTabEditorPanel';

function DocsPageInner() {
  const appBarContext = useContext(AppBarContext);
  const location = useLocation();
  const navigate = useNavigate();
  const { openDoc, closeTabByDocPath, updateTab, tabs, activeTabId, getActiveDocPath } = useDocTabContext();

  useEffect(() => {
    appBarContext.setTitle('Documents');
  }, [appBarContext]);

  // Derive active doc ID from URL
  const activeDocId = location.pathname.replace(/^\/docs\/?/, '') || null;

  // Sync URL to tab state on mount and URL change
  useEffect(() => {
    if (activeDocId) {
      const title = activeDocId.split('/').pop() || activeDocId;
      openDoc(activeDocId, title);
    }
  }, [activeDocId, openDoc]);

  // Sync tab changes to URL
  useEffect(() => {
    const currentDocPath = getActiveDocPath();
    if (currentDocPath && currentDocPath !== activeDocId) {
      navigate(`/docs/${currentDocPath}`, { replace: true });
    } else if (!currentDocPath && activeDocId) {
      navigate('/docs', { replace: true });
    }
  }, [activeTabId, tabs, getActiveDocPath, navigate, activeDocId]);

  const handleSelectDoc = useCallback((id: string, title: string) => {
    navigate(`/docs/${id}`);
  }, [navigate]);

  const handleDocCreated = useCallback((id: string) => {
    const title = id.split('/').pop() || id;
    navigate(`/docs/${id}`);
  }, [navigate]);

  const handleDocDeleted = useCallback((id: string) => {
    closeTabByDocPath(id);
  }, [closeTabByDocPath]);

  const handleDocRenamed = useCallback((oldId: string, newId: string) => {
    // Update the tab with the new path
    const tab = tabs.find(t => t.docPath === oldId);
    if (tab) {
      const newTitle = newId.split('/').pop() || newId;
      updateTab(tab.id, { docPath: newId, title: newTitle });
      navigate(`/docs/${newId}`, { replace: true });
    }
  }, [tabs, updateTab, navigate]);

  return (
    <div className="-m-4 md:-m-6 h-[calc(100vh-3.5rem)]">
      <SplitLayout
        defaultLeftWidth={25}
        minLeftWidth={15}
        maxLeftWidth={40}
        storageKey="docTreeWidth"
        emptyRightMessage="Select a document to start editing"
        leftPanel={
          <DocTreeSidebar
            activeDocId={activeDocId}
            onSelectDoc={handleSelectDoc}
            onDocCreated={handleDocCreated}
            onDocDeleted={handleDocDeleted}
            onDocRenamed={handleDocRenamed}
          />
        }
        rightPanel={<DocTabEditorPanel />}
      />
    </div>
  );
}

export default function DocsPage() {
  return (
    <DocTabProvider>
      <DocsPageInner />
    </DocTabProvider>
  );
}
