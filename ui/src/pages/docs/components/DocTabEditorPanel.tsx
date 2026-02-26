import { FileText } from 'lucide-react';
import { useDocTabContext } from '../contexts/DocTabContext';
import { DocTabBar } from './DocTabBar';
import { DocEditor } from './DocEditor';

export function DocTabEditorPanel() {
  const { tabs, activeTabId } = useDocTabContext();

  const activeTab = tabs.find(t => t.id === activeTabId);

  if (tabs.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-2 text-muted-foreground">
        <FileText className="w-10 h-10" />
        <p className="text-sm">Select a document to start editing</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <DocTabBar />
      <div className="flex-1 min-h-0">
        {activeTab ? (
          <DocEditor key={activeTab.docPath} docPath={activeTab.docPath} />
        ) : (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
            Select a tab
          </div>
        )}
      </div>
    </div>
  );
}
