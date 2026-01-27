import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { DAGContext } from '../types';

const TAB_STORAGE_KEY = 'dagu_dag_tabs';

interface StoredTab {
  id: string;
  fileName: string;
  title: string;
}

interface StoredTabState {
  tabs: StoredTab[];
  activeTabId: string | null;
}

/**
 * Hook to extract DAG context from:
 * 1. The current URL (for /dags/:fileName routes)
 * 2. The tab context stored in localStorage (for the split-panel DAGs view)
 */
export function useDagPageContext(): DAGContext | null {
  const location = useLocation();
  const [tabContext, setTabContext] = useState<DAGContext | null>(null);

  // Check URL for /dags/:fileName pattern
  const urlContext = (() => {
    const match = location.pathname.match(/\/dags\/([^/?]+)/);
    if (!match || !match[1]) {
      return null;
    }

    const fileName = decodeURIComponent(match[1]);
    const searchParams = new URLSearchParams(location.search);
    const dagRunId = searchParams.get('dagRunId') || undefined;

    return {
      dag_file: fileName,
      dag_run_id: dagRunId,
    };
  })();

  // Read from localStorage tab state
  useEffect(() => {
    const readTabState = () => {
      try {
        const stored = localStorage.getItem(TAB_STORAGE_KEY);
        if (stored) {
          const parsed: StoredTabState = JSON.parse(stored);
          if (parsed.activeTabId && parsed.tabs) {
            const activeTab = parsed.tabs.find(t => t.id === parsed.activeTabId);
            if (activeTab?.fileName) {
              setTabContext({
                dag_file: activeTab.fileName,
              });
              return;
            }
          }
        }
      } catch {
        // Ignore parse errors
      }
      setTabContext(null);
    };

    // Read immediately
    readTabState();

    // Listen for storage changes (in case tabs change in another component)
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === TAB_STORAGE_KEY) {
        readTabState();
      }
    };

    // Also poll periodically since storage events don't fire for same-tab changes
    const interval = setInterval(readTabState, 500);

    window.addEventListener('storage', handleStorageChange);
    return () => {
      window.removeEventListener('storage', handleStorageChange);
      clearInterval(interval);
    };
  }, []);

  // Prefer URL context if available, otherwise use tab context
  return urlContext || tabContext;
}
