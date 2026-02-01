import React, { createContext, useContext, useState } from 'react';

/**
 * Represents the current page's DAG/DAG-run context.
 * Used by the agent chat to understand what the user is viewing.
 */
export interface PageContextData {
  dagFile?: string;
  dagRunId?: string;
  dagRunName?: string;
  source: string;
}

interface PageContextType {
  context: PageContextData | null;
  setContext: (ctx: PageContextData | null) => void;
}

const PageContext = createContext<PageContextType | null>(null);

/** Accesses page context. Throws if used outside PageContextProvider. */
export function usePageContext(): PageContextType {
  const ctx = useContext(PageContext);
  if (!ctx) {
    throw new Error('usePageContext must be used within PageContextProvider');
  }
  return ctx;
}

/** Returns page context or null if not within provider. Safe to use anywhere. */
export function useOptionalPageContext(): PageContextType | null {
  return useContext(PageContext);
}

/** Provider that manages current page context. Wrap app root to enable tracking. */
export function PageContextProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const [context, setContext] = useState<PageContextData | null>(null);

  return (
    <PageContext.Provider value={{ context, setContext }}>
      {children}
    </PageContext.Provider>
  );
}
