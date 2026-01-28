import React, { createContext, useContext, useState, useCallback } from 'react';

/**
 * PageContextData represents the current page's DAG/DAG-run context.
 * This is used by the agent chat to know what the user is looking at.
 */
export interface PageContextData {
  /** DAG file name (e.g., "my-dag.yaml") */
  dagFile?: string;
  /** DAG run ID */
  dagRunId?: string;
  /** DAG run name (for display) */
  dagRunName?: string;
  /** Source component that set this context (for debugging) */
  source: string;
}

interface PageContextType {
  /** Current page context, or null if no DAG/run is being viewed */
  context: PageContextData | null;
  /** Set the current page context. Call with null to clear. */
  setContext: (ctx: PageContextData | null) => void;
}

const PageContext = createContext<PageContextType | null>(null);

/**
 * Hook to access and update the current page context.
 * Throws if used outside of PageContextProvider.
 */
export function usePageContext(): PageContextType {
  const ctx = useContext(PageContext);
  if (!ctx) {
    throw new Error('usePageContext must be used within PageContextProvider');
  }
  return ctx;
}

/**
 * Hook to access the current page context.
 * Returns null if not within provider (safe to use anywhere).
 */
export function useOptionalPageContext(): PageContextType | null {
  return useContext(PageContext);
}

/**
 * Provider component that manages the current page context.
 * Wrap your app with this to enable page context tracking.
 */
export function PageContextProvider({ children }: { children: React.ReactNode }) {
  const [context, setContextState] = useState<PageContextData | null>(null);

  const setContext = useCallback((ctx: PageContextData | null) => {
    setContextState(ctx);
  }, []);

  return (
    <PageContext.Provider value={{ context, setContext }}>
      {children}
    </PageContext.Provider>
  );
}
