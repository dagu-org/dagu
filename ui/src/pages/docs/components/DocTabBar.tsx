import React from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useDocTabContext } from '../contexts/DocTabContext';

export function DocTabBar() {
  const { tabs, activeTabId, closeTab, setActiveTab } = useDocTabContext();

  const handleKeyDown = (e: React.KeyboardEvent, tabId: string) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      setActiveTab(tabId);
    } else if (e.key === 'Delete' || e.key === 'Backspace') {
      e.preventDefault();
      closeTab(tabId);
    } else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault();
      const currentIndex = tabs.findIndex(t => t.id === tabId);
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

  if (tabs.length === 0) return null;

  return (
    <div
      className="flex items-end gap-0 bg-background border-b border-border overflow-x-auto overflow-y-hidden pt-3"
      role="tablist"
      aria-label="Document Tabs"
    >
      {tabs.map(tab => {
        const isActive = activeTabId === tab.id;
        return (
          <div
            key={tab.id}
            role="tab"
            aria-selected={isActive}
            tabIndex={isActive ? 0 : -1}
            onClick={() => setActiveTab(tab.id)}
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
            <span className="flex-1 text-sm font-medium truncate select-none">
              {tab.title || tab.docPath || 'Untitled'}
            </span>
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); closeTab(tab.id); }}
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
