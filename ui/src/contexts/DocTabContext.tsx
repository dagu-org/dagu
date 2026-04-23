// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useRef,
} from 'react';
import { useUnsavedChanges } from './UnsavedChangesContext';

export interface DocTab {
  id: string;
  docPath: string;
  title: string;
  workspace?: string | null;
}

interface DocTabContextType {
  tabs: DocTab[];
  activeTabId: string | null;
  openDoc: (docPath: string, title: string, workspace?: string | null) => void;
  closeTab: (tabId: string) => void;
  closeAllTabs: () => void;
  closeOtherTabs: (keepTabId: string) => void;
  setActiveTab: (tabId: string) => void;
  getActiveDocPath: () => string | null;
  updateTab: (
    tabId: string,
    updates: Partial<Pick<DocTab, 'docPath' | 'title' | 'workspace'>>
  ) => void;

  // Draft content persistence
  drafts: ReadonlyMap<string, string>;
  setDraft: (tabId: string, content: string) => void;
  clearDraft: (tabId: string) => void;
  getDraft: (tabId: string) => string | undefined;

  // Per-tab unsaved tracking
  unsavedTabIds: ReadonlySet<string>;
  markTabUnsaved: (tabId: string) => void;
  markTabSaved: (tabId: string) => void;
  isTabUnsaved: (tabId: string) => boolean;
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
  return `doc-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`;
}

function readStoredTabState(storageKey: string): StoredTabState | null {
  try {
    const stored =
      localStorage.getItem(storageKey) ||
      (storageKey !== STORAGE_KEY ? localStorage.getItem(STORAGE_KEY) : null);
    if (stored) {
      return JSON.parse(stored) as StoredTabState;
    }
  } catch {
    // Ignore parse errors
  }
  return null;
}

export function DocTabProvider({
  children,
  storageKey = STORAGE_KEY,
}: {
  children: React.ReactNode;
  storageKey?: string;
}) {
  const { setHasUnsavedChanges } = useUnsavedChanges();

  const [tabs, setTabs] = useState<DocTab[]>(() => {
    return readStoredTabState(storageKey)?.tabs || [];
  });

  const [activeTabId, setActiveTabId] = useState<string | null>(() => {
    const parsed = readStoredTabState(storageKey);
    if (
      parsed?.activeTabId &&
      parsed.tabs?.some((t) => t.id === parsed.activeTabId)
    ) {
      return parsed.activeTabId;
    }
    return null;
  });

  const [drafts, setDrafts] = useState<Map<string, string>>(new Map());
  const [unsavedTabIds, setUnsavedTabIds] = useState<Set<string>>(new Set());

  // Use ref to track tabs for use in callbacks without stale closures
  const tabsRef = useRef(tabs);
  tabsRef.current = tabs;
  const activeTabIdRef = useRef(activeTabId);
  activeTabIdRef.current = activeTabId;

  // Persist to localStorage
  useEffect(() => {
    try {
      const state: StoredTabState = { tabs, activeTabId };
      localStorage.setItem(storageKey, JSON.stringify(state));
    } catch {
      // Ignore persistence errors (quota/private mode)
    }
  }, [tabs, activeTabId, storageKey]);

  // Sync unsavedTabIds to UnsavedChangesContext
  useEffect(() => {
    setHasUnsavedChanges(unsavedTabIds.size > 0);
    return () => {
      setHasUnsavedChanges(false);
    };
  }, [unsavedTabIds, setHasUnsavedChanges]);

  const openDoc = useCallback((docPath: string, title: string, workspace?: string | null) => {
    // Check if already open
    const normalizedWorkspace = workspace ?? null;
    const existingTab = tabsRef.current.find(
      (t) =>
        t.docPath === docPath && (t.workspace ?? null) === normalizedWorkspace
    );
    if (existingTab) {
      setActiveTabId(existingTab.id);
      return;
    }

    // Create new tab
    const newTab: DocTab = {
      id: generateTabId(),
      docPath,
      title,
      workspace: normalizedWorkspace,
    };
    setTabs((prev) => [...prev, newTab]);
    setActiveTabId(newTab.id);
  }, []);

  const closeTab = useCallback((tabId: string) => {
    setTabs((prev) => {
      const newTabs = prev.filter((t) => t.id !== tabId);

      if (activeTabIdRef.current === tabId && newTabs.length > 0) {
        const closedIndex = prev.findIndex((t) => t.id === tabId);
        const newActiveIndex = Math.min(closedIndex, newTabs.length - 1);
        setActiveTabId(newTabs[newActiveIndex]?.id || null);
      } else if (newTabs.length === 0) {
        setActiveTabId(null);
      }

      return newTabs;
    });

    // Clear draft and unsaved state for closed tab
    setDrafts((prev) => {
      const next = new Map(prev);
      next.delete(tabId);
      return next;
    });
    setUnsavedTabIds((prev) => {
      const next = new Set(prev);
      next.delete(tabId);
      return next;
    });
  }, []);

  const closeAllTabs = useCallback(() => {
    setTabs([]);
    setActiveTabId(null);
    setDrafts(new Map());
    setUnsavedTabIds(new Set());
  }, []);

  const closeOtherTabs = useCallback((keepTabId: string) => {
    setTabs((prev) => prev.filter((t) => t.id === keepTabId));
    setActiveTabId(keepTabId);
    setDrafts((prev) => {
      const draft = prev.get(keepTabId);
      const next = new Map<string, string>();
      if (draft !== undefined) next.set(keepTabId, draft);
      return next;
    });
    setUnsavedTabIds((prev) => {
      const next = new Set<string>();
      if (prev.has(keepTabId)) next.add(keepTabId);
      return next;
    });
  }, []);

  const setActiveTab = useCallback((tabId: string) => {
    setActiveTabId(tabId);
  }, []);

  const getActiveDocPath = useCallback(() => {
    if (!activeTabIdRef.current) return null;
    const activeTab = tabsRef.current.find(
      (t) => t.id === activeTabIdRef.current
    );
    return activeTab?.docPath || null;
  }, []);

  const updateTab = useCallback(
    (
      tabId: string,
      updates: Partial<Pick<DocTab, 'docPath' | 'title' | 'workspace'>>
    ) => {
      setTabs((prev) =>
        prev.map((t) => (t.id === tabId ? { ...t, ...updates } : t))
      );
    },
    []
  );

  const setDraft = useCallback((tabId: string, content: string) => {
    setDrafts((prev) => {
      const next = new Map(prev);
      next.set(tabId, content);
      return next;
    });
  }, []);

  const clearDraft = useCallback((tabId: string) => {
    setDrafts((prev) => {
      const next = new Map(prev);
      next.delete(tabId);
      return next;
    });
  }, []);

  const getDraft = useCallback(
    (tabId: string) => {
      return drafts.get(tabId);
    },
    [drafts]
  );

  const markTabUnsaved = useCallback((tabId: string) => {
    setUnsavedTabIds((prev) => {
      if (prev.has(tabId)) return prev;
      const next = new Set(prev);
      next.add(tabId);
      return next;
    });
  }, []);

  const markTabSaved = useCallback((tabId: string) => {
    setUnsavedTabIds((prev) => {
      if (!prev.has(tabId)) return prev;
      const next = new Set(prev);
      next.delete(tabId);
      return next;
    });
  }, []);

  const isTabUnsaved = useCallback(
    (tabId: string) => {
      return unsavedTabIds.has(tabId);
    },
    [unsavedTabIds]
  );

  const value: DocTabContextType = {
    tabs,
    activeTabId,
    openDoc,
    closeTab,
    closeAllTabs,
    closeOtherTabs,
    setActiveTab,
    getActiveDocPath,
    updateTab,
    drafts,
    setDraft,
    clearDraft,
    getDraft,
    unsavedTabIds,
    markTabUnsaved,
    markTabSaved,
    isTabUnsaved,
  };

  return (
    <DocTabContext.Provider value={value}>{children}</DocTabContext.Provider>
  );
}
