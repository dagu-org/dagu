import { useDocTabContext } from '@/contexts/DocTabContext';
import { FileText } from 'lucide-react';
import React, { useCallback, useState } from 'react';
import ConfirmModal from '@/ui/ConfirmModal';
import DocEditor from './DocEditor';
import DocTabBar from './DocTabBar';

function DocTabEditorPanel() {
  const { tabs, activeTabId, closeTab, clearDraft, markTabSaved } =
    useDocTabContext();

  const [confirmCloseTabId, setConfirmCloseTabId] = useState<string | null>(null);

  const activeTab = activeTabId
    ? tabs.find((t) => t.id === activeTabId)
    : null;

  const handleCloseTabWithUnsaved = useCallback((tabId: string) => {
    setConfirmCloseTabId(tabId);
  }, []);

  const handleConfirmClose = useCallback(() => {
    if (confirmCloseTabId) {
      clearDraft(confirmCloseTabId);
      markTabSaved(confirmCloseTabId);
      closeTab(confirmCloseTabId);
      setConfirmCloseTabId(null);
    }
  }, [confirmCloseTabId, closeTab, clearDraft, markTabSaved]);

  const confirmTab = confirmCloseTabId
    ? tabs.find((t) => t.id === confirmCloseTabId)
    : null;

  if (tabs.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <FileText className="h-8 w-8 text-muted-foreground/50" />
        <p className="text-sm text-muted-foreground">
          Select a document to start editing.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <DocTabBar onCloseTabWithUnsaved={handleCloseTabWithUnsaved} />
      <div className="flex-1 overflow-hidden min-h-0">
        {activeTab ? (
          <DocEditor
            key={activeTab.id}
            tabId={activeTab.id}
            docPath={activeTab.docPath}
          />
        ) : (
          <div className="flex items-center justify-center h-full">
            <p className="text-sm text-muted-foreground">
              Select a tab to continue editing.
            </p>
          </div>
        )}
      </div>

      {/* Confirm close unsaved tab */}
      <ConfirmModal
        title="Unsaved Changes"
        buttonText="Discard"
        visible={!!confirmCloseTabId}
        dismissModal={() => setConfirmCloseTabId(null)}
        onSubmit={handleConfirmClose}
      >
        <p className="text-sm text-muted-foreground">
          You have unsaved changes in{' '}
          <strong>{confirmTab?.title || confirmTab?.docPath || 'this document'}</strong>.
          Discard changes?
        </p>
      </ConfirmModal>
    </div>
  );
}

export default DocTabEditorPanel;
