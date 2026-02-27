import { useDocTabContext } from '@/contexts/DocTabContext';
import { cn } from '@/lib/utils';
import { X } from 'lucide-react';
import React, { useCallback } from 'react';

type Props = {
  className?: string;
  onCloseTabWithUnsaved?: (tabId: string) => void;
};

function DocTabBar({ className, onCloseTabWithUnsaved }: Props) {
  const { tabs, activeTabId, closeTab, setActiveTab, isTabUnsaved } =
    useDocTabContext();

  const handleTabClick = (tabId: string) => {
    setActiveTab(tabId);
  };

  const handleCloseTab = useCallback(
    (e: React.MouseEvent, tabId: string) => {
      e.stopPropagation();
      if (isTabUnsaved(tabId) && onCloseTabWithUnsaved) {
        onCloseTabWithUnsaved(tabId);
      } else {
        closeTab(tabId);
      }
    },
    [closeTab, isTabUnsaved, onCloseTabWithUnsaved]
  );

  const handleKeyDown = (e: React.KeyboardEvent, tabId: string) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      setActiveTab(tabId);
    } else if (e.key === 'Delete' || e.key === 'Backspace') {
      e.preventDefault();
      if (isTabUnsaved(tabId) && onCloseTabWithUnsaved) {
        onCloseTabWithUnsaved(tabId);
      } else {
        closeTab(tabId);
      }
    } else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault();
      const currentIndex = tabs.findIndex((tab) => tab.id === tabId);
      if (currentIndex === -1) return;

      let nextIndex: number;
      if (e.key === 'ArrowRight') {
        nextIndex = currentIndex + 1 >= tabs.length ? 0 : currentIndex + 1;
      } else {
        nextIndex = currentIndex - 1 < 0 ? tabs.length - 1 : currentIndex - 1;
      }

      const nextTab = tabs[nextIndex];
      if (nextTab) {
        setActiveTab(nextTab.id);
      }
    }
  };

  return (
    <div
      className={cn(
        'flex items-end gap-0 bg-background border-b border-border overflow-x-auto overflow-y-hidden pt-3',
        className
      )}
      role="tablist"
      aria-label="Document Tabs"
    >
      {tabs.map((tab) => {
        const isActive = activeTabId === tab.id;
        const unsaved = isTabUnsaved(tab.id);

        return (
          <div
            key={tab.id}
            role="tab"
            aria-selected={isActive}
            tabIndex={isActive ? 0 : -1}
            onClick={() => handleTabClick(tab.id)}
            onKeyDown={(e) => handleKeyDown(e, tab.id)}
            className={cn(
              'group relative flex items-center gap-2 h-8 px-3 min-w-[120px] max-w-[200px]',
              'border-b-2 transition-all duration-150 cursor-pointer shrink-0',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1',
              isActive
                ? 'border-primary bg-transparent text-foreground'
                : 'border-transparent bg-transparent text-text-secondary hover:bg-muted hover:text-foreground'
            )}
          >
            {/* Unsaved indicator */}
            {unsaved && (
              <span className="h-1.5 w-1.5 rounded-full bg-amber-500 shrink-0" />
            )}

            {/* Tab Label */}
            <span className="flex-1 text-sm font-medium truncate select-none">
              {tab.title || tab.docPath || 'Untitled'}
            </span>

            {/* Close Button */}
            <button
              type="button"
              onClick={(e) => handleCloseTab(e, tab.id)}
              className={cn(
                'flex items-center justify-center shrink-0 w-4 h-4 rounded-sm',
                'transition-all duration-150',
                'hover:bg-muted-foreground/20 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
                'opacity-0 group-hover:opacity-100',
                isActive && 'opacity-100'
              )}
              aria-label={`Close ${tab.title || tab.docPath}`}
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        );
      })}
    </div>
  );
}

export default DocTabBar;
