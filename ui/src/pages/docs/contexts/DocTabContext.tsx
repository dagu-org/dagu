import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';

export interface DocTab {
  id: string;
  docPath: string;
  title: string;
}

interface DocTabContextType {
  tabs: DocTab[];
  activeTabId: string | null;
  openDoc: (docPath: string, title: string) => void;
  closeTab: (tabId: string) => void;
  closeTabByDocPath: (docPath: string) => void;
  setActiveTab: (tabId: string) => void;
  getActiveDocPath: () => string | null;
  updateTab: (tabId: string, updates: Partial<Pick<DocTab, 'docPath' | 'title'>>) => void;
}

const STORAGE_KEY = 'dagu_doc_tabs';

const DocTabContext = createContext<DocTabContextType | null>(null);

export function useDocTabContext() {
  const context = useContext(DocTabContext);
  if (!context) {
    throw new Error('useDocTabContext must be used within a DocTabProvider');
  }
  return context;
}

interface StoredTabState {
  tabs: DocTab[];
  activeTabId: string | null;
}

function generateTabId(): string {
  return `doc-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

export function DocTabProvider({ children }: { children: React.ReactNode }) {
  const [tabs, setTabs] = useState<DocTab[]>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: StoredTabState = JSON.parse(stored);
        return parsed.tabs || [];
      }
    } catch {
      // Ignore parse errors
    }
    return [];
  });

  const [activeTabId, setActiveTabId] = useState<string | null>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: StoredTabState = JSON.parse(stored);
        if (parsed.activeTabId && parsed.tabs?.some(t => t.id === parsed.activeTabId)) {
          return parsed.activeTabId;
        }
      }
    } catch {
      // Ignore parse errors
    }
    return null;
  });

  // Persist to localStorage
  useEffect(() => {
    const state: StoredTabState = { tabs, activeTabId };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }, [tabs, activeTabId]);

  const openDoc = useCallback((docPath: string, title: string) => {
    setTabs(prev => {
      const existing = prev.find(t => t.docPath === docPath);
      if (existing) {
        setActiveTabId(existing.id);
        return prev;
      }
      const newTab: DocTab = { id: generateTabId(), docPath, title };
      setActiveTabId(newTab.id);
      return [...prev, newTab];
    });
  }, []);

  const closeTab = useCallback((tabId: string) => {
    setTabs(prev => {
      const newTabs = prev.filter(t => t.id !== tabId);

      if (activeTabId === tabId && newTabs.length > 0) {
        const closedIndex = prev.findIndex(t => t.id === tabId);
        const newActiveIndex = Math.min(closedIndex, newTabs.length - 1);
        setActiveTabId(newTabs[newActiveIndex]?.id || null);
      } else if (newTabs.length === 0) {
        setActiveTabId(null);
      }

      return newTabs;
    });
  }, [activeTabId]);

  const closeTabByDocPath = useCallback((docPath: string) => {
    const tab = tabs.find(t => t.docPath === docPath);
    if (tab) {
      closeTab(tab.id);
    }
  }, [tabs, closeTab]);

  const setActiveTab = useCallback((tabId: string) => {
    setActiveTabId(tabId);
  }, []);

  const getActiveDocPath = useCallback(() => {
    if (!activeTabId) return null;
    const activeTab = tabs.find(t => t.id === activeTabId);
    return activeTab?.docPath || null;
  }, [tabs, activeTabId]);

  const updateTab = useCallback((tabId: string, updates: Partial<Pick<DocTab, 'docPath' | 'title'>>) => {
    setTabs(prev => prev.map(t =>
      t.id === tabId ? { ...t, ...updates } : t
    ));
  }, []);

  const value: DocTabContextType = {
    tabs,
    activeTabId,
    openDoc,
    closeTab,
    closeTabByDocPath,
    setActiveTab,
    getActiveDocPath,
    updateTab,
  };

  return (
    <DocTabContext.Provider value={value}>
      {children}
    </DocTabContext.Provider>
  );
}
