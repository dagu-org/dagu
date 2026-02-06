import React, { createContext, useContext, useCallback, useEffect, useMemo, useState } from 'react';
import { useSWRConfig } from 'swr';
import { useClient } from '../hooks/api';

const NAMESPACE_STORAGE_KEY = 'dagu-selected-namespace';
export const ALL_NAMESPACES = '__all__';

export type NamespaceInfo = {
  name: string;
  description?: string;
};

type NamespaceContextType = {
  namespaces: NamespaceInfo[];
  selectedNamespace: string; // namespace name or ALL_NAMESPACES
  selectNamespace: (ns: string) => void;
  isAllNamespaces: boolean;
  isLoading: boolean;
};

const NamespaceContext = createContext<NamespaceContextType>({
  namespaces: [],
  selectedNamespace: 'default',
  selectNamespace: () => {},
  isAllNamespaces: false,
  isLoading: true,
});

export function NamespaceProvider({ children }: { children: React.ReactNode }) {
  const client = useClient();
  const { mutate: globalMutate } = useSWRConfig();
  const [namespaces, setNamespaces] = useState<NamespaceInfo[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string>(
    () => localStorage.getItem(NAMESPACE_STORAGE_KEY) || 'default'
  );
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function fetchNamespaces() {
      try {
        const { data } = await client.GET('/namespaces');
        if (!cancelled && data?.namespaces) {
          const nsList = data.namespaces.map((ns) => ({
            name: ns.name,
            description: ns.description,
          }));
          setNamespaces(nsList);

          // Validate stored selection against available namespaces
          const stored = localStorage.getItem(NAMESPACE_STORAGE_KEY);
          if (stored && stored !== ALL_NAMESPACES) {
            const valid = nsList.some((ns) => ns.name === stored);
            if (!valid) {
              setSelectedNamespace('default');
              localStorage.setItem(NAMESPACE_STORAGE_KEY, 'default');
            }
          }
        }
      } catch {
        // If fetch fails (e.g., auth not configured), fall back to default
        setNamespaces([{ name: 'default' }]);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }
    fetchNamespaces();
    return () => { cancelled = true; };
  }, [client]);

  const selectNamespace = useCallback((ns: string) => {
    setSelectedNamespace(ns);
    localStorage.setItem(NAMESPACE_STORAGE_KEY, ns);
    // Revalidate all SWR queries so views refetch with new namespace scope
    globalMutate(() => true, undefined, { revalidate: true });
  }, [globalMutate]);

  const isAllNamespaces = selectedNamespace === ALL_NAMESPACES;

  const value = useMemo(
    () => ({
      namespaces,
      selectedNamespace,
      selectNamespace,
      isAllNamespaces,
      isLoading,
    }),
    [namespaces, selectedNamespace, selectNamespace, isAllNamespaces, isLoading]
  );

  return (
    <NamespaceContext.Provider value={value}>
      {children}
    </NamespaceContext.Provider>
  );
}

export function useNamespace(): NamespaceContextType {
  return useContext(NamespaceContext);
}
